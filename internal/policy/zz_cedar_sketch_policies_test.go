// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// SKETCH (cedar-integration-sketch, not for merge): evaluates the demo Cedar
// policies in demo/cedar against hand-built facts. Nothing here touches git or
// gittuf's native verification; the facts are what gittuf would compute and
// pass to Cedar. Every rule is a forbid layered on permit-all.cedar, because
// gittuf's final verdict is native AND Cedar.

package policy

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/cedar-policy/cedar-go"
	"github.com/cedar-policy/cedar-go/types"
	xast "github.com/cedar-policy/cedar-go/x/exp/ast"
	"github.com/cedar-policy/cedar-go/x/exp/schema"
	"github.com/cedar-policy/cedar-go/x/exp/schema/validate"
	"github.com/stretchr/testify/require"
)

const (
	sketchPolicyDir        = "../../demo/cedar"
	sketchPolicySchemaFile = "cedar.cedarschema"

	sketchPolicyPermitAll = "permit-all.cedar"
	sketchPolicyRule2     = "rule2-required-group.cedar"
	sketchPolicyRule3     = "rule3-no-self-approval.cedar"
	sketchPolicyRule4     = "rule4-separation-of-duties.cedar"
	sketchPolicyRule5     = "rule5-forbid-principal.cedar"

	sketchPolicyMain = "refs/heads/main"
)

var sketchPolicyRuleFiles = []string{sketchPolicyRule2, sketchPolicyRule3, sketchPolicyRule4, sketchPolicyRule5}

// sketchPolicyPR mirrors the schema's PullRequest record. author is the
// principal ID gittuf matched for authorIdentity, or "" when none matched.
type sketchPolicyPR struct {
	number         int64
	authorIdentity string
	author         string
	labels         []string
}

// sketchPolicyFacts is a hand-built stand-in for the facts gittuf computes.
type sketchPolicyFacts struct {
	action             string
	target             string
	credited           []string
	approvers          []string
	approverIdentities []string
	verifiedUsing      string
	pr                 *sketchPolicyPR
}

func sketchPolicyCompliantFacts() sketchPolicyFacts {
	return sketchPolicyFacts{
		action:             "updateRef",
		target:             sketchPolicyMain,
		credited:           []string{"alice", "rm-1", "qa-1", "sec-1"},
		approvers:          []string{"qa-1", "sec-1"},
		approverIdentities: []string{"qa-1+301", "sec-1+401"},
		verifiedUsing:      "protect-main",
		pr: &sketchPolicyPR{
			number:         7,
			authorIdentity: "alice+101",
			author:         "alice",
			labels:         []string{"ready"},
		},
	}
}

// sketchPolicyGroupMembers is the hand-built group membership. In gittuf it
// comes from tufv02.Person.Custom["cedar-groups"] in the signed policy.
var sketchPolicyGroupMembers = map[string][]string{
	"dev":              {"dev-1", "dev-2", "dev-3", "dev-4", "dev-5", "alice", "mallory"},
	"qa":               {"qa-1", "qa-2"},
	"security":         {"sec-1"},
	"release-managers": {"rm-1"},
}

func sketchPolicyPrincipalUID(id string) types.EntityUID {
	return types.NewEntityUID("Gittuf::Principal", types.String(id))
}

func sketchPolicyGroupUID(id string) types.EntityUID {
	return types.NewEntityUID("Gittuf::Group", types.String(id))
}

func sketchPolicyTargetUID(scheme, name string) types.EntityUID {
	return types.NewEntityUID("Gittuf::Target", types.String(scheme+":"+name))
}

func sketchPolicyChangeUID() types.EntityUID {
	return types.NewEntityUID("Gittuf::Change", "rsl-entry-0001")
}

// sketchPolicyEntities builds groups, principals (with group parents), the
// change, and one target entity per name.
func sketchPolicyEntities(targets ...string) types.EntityMap {
	parents := map[string][]types.EntityUID{}
	entities := types.EntityMap{}
	for group, members := range sketchPolicyGroupMembers {
		uid := sketchPolicyGroupUID(group)
		entities[uid] = types.Entity{UID: uid}
		for _, member := range members {
			parents[member] = append(parents[member], uid)
		}
	}
	for member, groups := range parents {
		uid := sketchPolicyPrincipalUID(member)
		entities[uid] = types.Entity{UID: uid, Parents: types.NewEntityUIDSet(groups...)}
	}
	change := sketchPolicyChangeUID()
	entities[change] = types.Entity{UID: change}
	for _, name := range targets {
		uid := sketchPolicyTargetUID("git", name)
		entities[uid] = types.Entity{
			UID: uid,
			Attributes: types.NewRecord(types.RecordMap{
				"scheme": types.String("git"),
				"name":   types.String(name),
			}),
		}
	}
	return entities
}

// sketchPolicyCreditedGroups derives creditedGroups from the credited
// principals' group parents in the entity map.
func sketchPolicyCreditedGroups(entities types.EntityMap, credited []string) []types.Value {
	seen := map[types.EntityUID]bool{}
	groups := []types.Value{}
	for _, id := range credited {
		entity, ok := entities[sketchPolicyPrincipalUID(id)]
		if !ok {
			continue
		}
		for parent := range entity.Parents.All() {
			if !seen[parent] {
				seen[parent] = true
				groups = append(groups, parent)
			}
		}
	}
	return groups
}

func sketchPolicyPrincipalSet(ids []string) types.Set {
	values := make([]types.Value, 0, len(ids))
	for _, id := range ids {
		values = append(values, sketchPolicyPrincipalUID(id))
	}
	return types.NewSet(values...)
}

func sketchPolicyStringSet(items []string) types.Set {
	values := make([]types.Value, 0, len(items))
	for _, item := range items {
		values = append(values, types.String(item))
	}
	return types.NewSet(values...)
}

func sketchPolicyRequest(facts sketchPolicyFacts, entities types.EntityMap) cedar.Request {
	context := types.RecordMap{
		"mergeable":          types.False,
		"credited":           sketchPolicyPrincipalSet(facts.credited),
		"creditedGroups":     types.NewSet(sketchPolicyCreditedGroups(entities, facts.credited)...),
		"approvers":          sketchPolicyPrincipalSet(facts.approvers),
		"approverIdentities": sketchPolicyStringSet(facts.approverIdentities),
		"verifiedUsing":      types.String(facts.verifiedUsing),
	}
	if facts.pr != nil {
		pr := types.RecordMap{
			"number":         types.Long(facts.pr.number),
			"authorIdentity": types.String(facts.pr.authorIdentity),
			"labels":         sketchPolicyStringSet(facts.pr.labels),
		}
		if facts.pr.author != "" {
			pr["author"] = sketchPolicyPrincipalUID(facts.pr.author)
		}
		context["pr"] = types.NewRecord(pr)
	}
	return cedar.Request{
		Principal: sketchPolicyChangeUID(),
		Action:    types.NewEntityUID("Gittuf::Action", types.String(facts.action)),
		Resource:  sketchPolicyTargetUID("git", facts.target),
		Context:   types.NewRecord(context),
	}
}

func sketchPolicyValidator(t *testing.T) *validate.Validator {
	t.Helper()
	b, err := os.ReadFile(sketchPolicySchemaFile)
	require.Nil(t, err)
	var s schema.Schema
	require.Nil(t, s.UnmarshalCedar(b))
	rs, err := s.Resolve()
	require.Nil(t, err)
	return validate.New(rs, validate.WithStrict())
}

// sketchPolicyParse parses a Cedar document and names each policy by its
// @id annotation so reasons are stable across files and concatenation.
func sketchPolicyParse(t *testing.T, name string, document []byte) []sketchPolicyNamed {
	t.Helper()
	list, err := cedar.NewPolicyListFromBytes(name, document)
	require.Nil(t, err, name)
	named := make([]sketchPolicyNamed, 0, len(list))
	for _, p := range list {
		id, ok := p.Annotations()["id"]
		require.True(t, ok, "policy in %s has no @id", name)
		named = append(named, sketchPolicyNamed{id: cedar.PolicyID(id), policy: p})
	}
	return named
}

type sketchPolicyNamed struct {
	id     cedar.PolicyID
	policy *cedar.Policy
}

func sketchPolicyRead(t *testing.T, file string) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(sketchPolicyDir, file))
	require.Nil(t, err)
	return b
}

// sketchPolicyLoad builds a policy set from the given demo files, rejecting
// duplicate IDs.
func sketchPolicyLoad(t *testing.T, files ...string) *cedar.PolicySet {
	t.Helper()
	ps := cedar.NewPolicySet()
	for _, file := range files {
		for _, n := range sketchPolicyParse(t, file, sketchPolicyRead(t, file)) {
			require.True(t, ps.Add(n.id, n.policy), "duplicate policy id %s", n.id)
		}
	}
	return ps
}

// sketchPolicyLoadConcatenated concatenates the files into one document and
// parses it as a single policy set, as a deployment would.
func sketchPolicyLoadConcatenated(t *testing.T, files ...string) *cedar.PolicySet {
	t.Helper()
	var document []byte
	for _, file := range files {
		document = append(document, sketchPolicyRead(t, file)...)
		document = append(document, '\n')
	}
	ps := cedar.NewPolicySet()
	for _, n := range sketchPolicyParse(t, "all.cedar", document) {
		require.True(t, ps.Add(n.id, n.policy), "duplicate policy id %s", n.id)
	}
	return ps
}

// sketchPolicyAllows is the fail-closed wrapper: any evaluation error denies,
// because cedar-go skips a policy whose condition errors.
func sketchPolicyAllows(decision cedar.Decision, diag cedar.Diagnostic) bool {
	return decision == cedar.Allow && len(diag.Errors) == 0
}

func sketchPolicyReasons(diag cedar.Diagnostic) []string {
	reasons := make([]string, 0, len(diag.Reasons))
	for _, r := range diag.Reasons {
		reasons = append(reasons, string(r.PolicyID))
	}
	sort.Strings(reasons)
	return reasons
}

// sketchPolicyEvaluate validates the request and entities against the schema
// and then authorizes.
func sketchPolicyEvaluate(t *testing.T, v *validate.Validator, ps *cedar.PolicySet, facts sketchPolicyFacts) (cedar.Decision, cedar.Diagnostic) {
	t.Helper()
	entities := sketchPolicyEntities(facts.target)
	require.Nil(t, v.Entities(entities))
	req := sketchPolicyRequest(facts, entities)
	require.Nil(t, v.Request(req))
	return cedar.Authorize(ps, entities, req)
}

func TestCedarSketchPolicyFiles(t *testing.T) {
	v := sketchPolicyValidator(t)

	t.Run("strict validation", func(t *testing.T) {
		for _, file := range append([]string{sketchPolicyPermitAll}, sketchPolicyRuleFiles...) {
			for _, n := range sketchPolicyParse(t, file, sketchPolicyRead(t, file)) {
				err := v.Policy(string(n.id), (*xast.Policy)(n.policy.AST()))
				require.Nil(t, err, "%s: %s", file, n.id)
				effect := "forbid"
				if n.policy.Effect() == cedar.Permit {
					effect = "permit"
				}
				t.Logf("validated %s (%s, %s): ok", n.id, file, effect)
			}
		}
	})

	t.Run("fact validation is not vacuous", func(t *testing.T) {
		entities := sketchPolicyEntities(sketchPolicyMain)
		target := sketchPolicyTargetUID("git", sketchPolicyMain)
		entities[target] = types.Entity{UID: target} // missing scheme and name
		err := v.Entities(entities)
		require.NotNil(t, err)
		t.Logf("target without attributes rejected: %v", err)

		req := sketchPolicyRequest(sketchPolicyCompliantFacts(), sketchPolicyEntities(sketchPolicyMain))
		req.Context = types.NewRecord(types.RecordMap{"mergeable": types.True})
		err = v.Request(req)
		require.NotNil(t, err)
		t.Logf("incomplete context rejected: %v", err)
	})

	t.Run("concatenated policy set", func(t *testing.T) {
		ps := sketchPolicyLoadConcatenated(t, append([]string{sketchPolicyPermitAll}, sketchPolicyRuleFiles...)...)
		ids := []string{}
		for id := range ps.All() {
			ids = append(ids, string(id))
		}
		sort.Strings(ids)
		t.Logf("concatenated policy ids: %v", ids)
		require.Len(t, ids, 6)
	})

	tests := map[string]struct {
		file      string
		violating func() sketchPolicyFacts
		reasons   []string
	}{
		"rule2 required group": {
			file: sketchPolicyRule2,
			violating: func() sketchPolicyFacts {
				f := sketchPolicyCompliantFacts()
				f.credited = []string{"alice", "qa-1", "sec-1"} // no release manager
				return f
			},
			reasons: []string{"rule2-required-group"},
		},
		"rule3a no self-approval, principal match": {
			file: sketchPolicyRule3,
			violating: func() sketchPolicyFacts {
				// alice authored as alice+101 and approved from a second
				// linked account; only the principal match catches it.
				f := sketchPolicyCompliantFacts()
				f.approvers = append(f.approvers, "alice")
				f.approverIdentities = append(f.approverIdentities, "alice-alt+102")
				return f
			},
			reasons: []string{"rule3a-no-self-approval-principal"},
		},
		"rule3b no self-approval, identity match": {
			file: sketchPolicyRule3,
			violating: func() sketchPolicyFacts {
				// The author is not a policy person, so pr.author is absent.
				f := sketchPolicyCompliantFacts()
				f.pr.authorIdentity = "outsider+900"
				f.pr.author = ""
				f.approverIdentities = append(f.approverIdentities, "outsider+900")
				return f
			},
			reasons: []string{"rule3b-no-self-approval-identity"},
		},
		"rule4 separation of duties": {
			file: sketchPolicyRule4,
			violating: func() sketchPolicyFacts {
				f := sketchPolicyCompliantFacts()
				f.credited = []string{"alice", "rm-1", "qa-1"} // no security
				return f
			},
			reasons: []string{"rule4-separation-of-duties"},
		},
		"rule5 forbid principal": {
			file: sketchPolicyRule5,
			violating: func() sketchPolicyFacts {
				f := sketchPolicyCompliantFacts()
				f.credited = append(f.credited, "mallory")
				return f
			},
			reasons: []string{"rule5-forbid-principal"},
		},
	}

	names := make([]string, 0, len(tests))
	for name := range tests {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		test := tests[name]
		t.Run(name, func(t *testing.T) {
			permitAll := sketchPolicyLoad(t, sketchPolicyPermitAll)
			withRule := sketchPolicyLoad(t, sketchPolicyPermitAll, test.file)

			decision, diag := sketchPolicyEvaluate(t, v, permitAll, test.violating())
			require.Empty(t, diag.Errors)
			require.Equal(t, cedar.Allow, decision)
			require.Equal(t, []string{"permit-all"}, sketchPolicyReasons(diag))
			t.Logf("permit-all, violating facts: %s %v", decision, sketchPolicyReasons(diag))

			decision, diag = sketchPolicyEvaluate(t, v, withRule, test.violating())
			require.Empty(t, diag.Errors)
			require.Equal(t, cedar.Deny, decision)
			require.Equal(t, test.reasons, sketchPolicyReasons(diag))
			require.False(t, sketchPolicyAllows(decision, diag))
			t.Logf("permit-all + %s, violating facts: %s %v", test.file, decision, sketchPolicyReasons(diag))

			decision, diag = sketchPolicyEvaluate(t, v, withRule, sketchPolicyCompliantFacts())
			require.Empty(t, diag.Errors)
			require.Equal(t, cedar.Allow, decision)
			require.True(t, sketchPolicyAllows(decision, diag))
			t.Logf("permit-all + %s, compliant facts: %s %v", test.file, decision, sketchPolicyReasons(diag))

			// With every rule deployed, the violating facts trip only this rule.
			all := sketchPolicyLoadConcatenated(t, append([]string{sketchPolicyPermitAll}, sketchPolicyRuleFiles...)...)
			decision, diag = sketchPolicyEvaluate(t, v, all, test.violating())
			require.Empty(t, diag.Errors)
			require.Equal(t, cedar.Deny, decision)
			require.Equal(t, test.reasons, sketchPolicyReasons(diag))
			t.Logf("all rules, violating facts: %s %v", decision, sketchPolicyReasons(diag))
		})
	}

	t.Run("all rules, compliant facts", func(t *testing.T) {
		all := sketchPolicyLoadConcatenated(t, append([]string{sketchPolicyPermitAll}, sketchPolicyRuleFiles...)...)
		decision, diag := sketchPolicyEvaluate(t, v, all, sketchPolicyCompliantFacts())
		require.Empty(t, diag.Errors)
		require.Equal(t, cedar.Allow, decision)
		require.True(t, sketchPolicyAllows(decision, diag))
		t.Logf("all rules, compliant facts: %s %v", decision, sketchPolicyReasons(diag))

		// No PR context at all (a direct push): pr is absent, rule 3 must not
		// error.
		noPR := sketchPolicyCompliantFacts()
		noPR.pr = nil
		decision, diag = sketchPolicyEvaluate(t, v, all, noPR)
		require.Empty(t, diag.Errors)
		require.Equal(t, cedar.Allow, decision)
		t.Logf("all rules, compliant facts without pr: %s %v", decision, sketchPolicyReasons(diag))
	})

	t.Run("main scoping", func(t *testing.T) {
		// Facts that violate rules 2, 4 and 5 on main are allowed elsewhere.
		all := sketchPolicyLoadConcatenated(t, append([]string{sketchPolicyPermitAll}, sketchPolicyRuleFiles...)...)
		f := sketchPolicyCompliantFacts()
		f.credited = []string{"mallory"}
		f.target = "refs/heads/feature"
		decision, diag := sketchPolicyEvaluate(t, v, all, f)
		require.Empty(t, diag.Errors)
		require.Equal(t, cedar.Allow, decision)
		t.Logf("all rules, refs/heads/feature with only mallory credited: %s %v", decision, sketchPolicyReasons(diag))

		f.target = sketchPolicyMain
		decision, diag = sketchPolicyEvaluate(t, v, all, f)
		require.Empty(t, diag.Errors)
		require.Equal(t, cedar.Deny, decision)
		require.Equal(t, []string{"rule2-required-group", "rule4-separation-of-duties", "rule5-forbid-principal"}, sketchPolicyReasons(diag))
		t.Logf("all rules, refs/heads/main with only mallory credited: %s %v", decision, sketchPolicyReasons(diag))
	})

	t.Run("fail closed on evaluation error", func(t *testing.T) {
		const unguarded = `@id("unguarded-labels")
forbid (principal, action, resource)
when { context.pr.labels.contains("do-not-merge") };`

		named := sketchPolicyParse(t, "unguarded.cedar", []byte(unguarded))
		require.Len(t, named, 1)
		err := v.Policy(string(named[0].id), (*xast.Policy)(named[0].policy.AST()))
		require.NotNil(t, err)
		t.Logf("strict validation rejects unguarded policy: %v", err)

		ps := sketchPolicyLoad(t, sketchPolicyPermitAll)
		require.True(t, ps.Add(named[0].id, named[0].policy))

		f := sketchPolicyCompliantFacts()
		f.pr = nil // the forbid's condition errors instead of evaluating to true
		decision, diag := sketchPolicyEvaluate(t, v, ps, f)
		require.Equal(t, cedar.Allow, decision)
		require.Len(t, diag.Errors, 1)
		require.Equal(t, cedar.PolicyID("unguarded-labels"), diag.Errors[0].PolicyID)
		require.False(t, sketchPolicyAllows(decision, diag))
		t.Logf("raw cedar-go: %s, errors=%v; fail-closed wrapper allow=%t", decision, sketchPolicyErrorStrings(diag), sketchPolicyAllows(decision, diag))
	})
}

func sketchPolicyErrorStrings(diag cedar.Diagnostic) []string {
	out := []string{}
	for _, e := range diag.Errors {
		out = append(out, fmt.Sprintf("%s: %s", e.PolicyID, e.Message))
	}
	return out
}
