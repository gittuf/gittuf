// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cedarsketch

package policy

// SKETCH EXPERIMENT (cedar-integration-sketch, throwaway, not for merge).
// Validates the demo Cedar policies against the sketch schema with cedar-go
// v1.8.0 and evaluates them on hand-built facts.
//
// cedar-go is NOT in the repo's go.mod. Run with a scratch modfile:
//   go test -tags cedarsketch -modfile=<scratch>/go.mod -run TestCedarSketchPolicies ./internal/policy/

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/cedar-policy/cedar-go"
	cedarxast "github.com/cedar-policy/cedar-go/x/exp/ast"
	cedarschema "github.com/cedar-policy/cedar-go/x/exp/schema"
	cedarvalidate "github.com/cedar-policy/cedar-go/x/exp/schema/validate"
	"github.com/stretchr/testify/require"
)

func sketchV0CedarSketchDir() string {
	if d := os.Getenv("CEDAR_SKETCH_DIR"); d != "" {
		return d
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, "cedar-sketch-artifacts", "cedar")
}

// sketchV0CedarFacts is the Go side of the schema in gittuf.cedarschema.
type sketchV0CedarFacts struct {
	Change             string
	Action             string // updateRef | updateTag | modifyFile
	Scheme, Name       string
	Mergeable          bool
	Signers            []string
	Approvers          []string
	ApproverIdentities []string
	Dismissed          []string
	MatchedRules       []string
	VerifiedUsing      string
	PR                 *sketchV0CedarPR
}

type sketchV0CedarPR struct {
	Number           int64
	AuthorIdentity   string
	Author           string // mapped principal ID, "" if unmapped
	Labels           []string
	Draft            bool
	MergedByIdentity string
}

func sketchV0PrincipalUID(id string) cedar.EntityUID {
	return cedar.NewEntityUID("Gittuf::Principal", cedar.String(id))
}

func sketchV0PrincipalSet(ids []string) cedar.Set {
	vals := make([]cedar.Value, 0, len(ids))
	for _, id := range ids {
		vals = append(vals, sketchV0PrincipalUID(id))
	}
	return cedar.NewSet(vals...)
}

func sketchV0StringSet(ss []string) cedar.Set {
	vals := make([]cedar.Value, 0, len(ss))
	for _, s := range ss {
		vals = append(vals, cedar.String(s))
	}
	return cedar.NewSet(vals...)
}

// sketchV0BuildCedarRequest turns facts into a request and an entity map. base holds
// the policy-data entities (groups and their members).
func sketchV0BuildCedarRequest(f sketchV0CedarFacts, base cedar.EntityMap) (cedar.Request, cedar.EntityMap) {
	entities := cedar.EntityMap{}
	for k, v := range base {
		entities[k] = v
	}

	target := cedar.NewEntityUID("Gittuf::Target", cedar.String(f.Scheme+":"+f.Name))
	entities[target] = cedar.Entity{UID: target, Attributes: cedar.NewRecord(cedar.RecordMap{
		"scheme": cedar.String(f.Scheme),
		"name":   cedar.String(f.Name),
	})}
	change := cedar.NewEntityUID("Gittuf::Change", cedar.String(f.Change))
	entities[change] = cedar.Entity{UID: change}

	credited := map[string]bool{}
	for _, s := range append(append([]string{}, f.Signers...), f.Approvers...) {
		credited[s] = true
	}
	creditedIDs := []string{}
	groups := map[cedar.EntityUID]bool{}
	for id := range credited {
		creditedIDs = append(creditedIDs, id)
		uid := sketchV0PrincipalUID(id)
		if _, ok := entities[uid]; !ok {
			entities[uid] = cedar.Entity{UID: uid}
		}
		// Ancestor closure (one level is enough for the sketch's entities).
		for p := range entities[uid].Parents.All() {
			groups[p] = true
		}
	}
	sort.Strings(creditedIDs)
	groupVals := []cedar.Value{}
	for g := range groups {
		groupVals = append(groupVals, g)
	}

	ctx := cedar.RecordMap{
		"mergeable":           cedar.Boolean(f.Mergeable),
		"signers":             sketchV0PrincipalSet(f.Signers),
		"approvers":           sketchV0PrincipalSet(f.Approvers),
		"approverIdentities":  sketchV0StringSet(f.ApproverIdentities),
		"dismissedIdentities": sketchV0StringSet(f.Dismissed),
		"credited":            sketchV0PrincipalSet(creditedIDs),
		"creditedGroups":      cedar.NewSet(groupVals...),
		"matchedRules":        sketchV0StringSet(f.MatchedRules),
		"verifiedUsing":       cedar.String(f.VerifiedUsing),
	}
	if f.PR != nil {
		pr := cedar.RecordMap{
			"number":         cedar.Long(f.PR.Number),
			"authorIdentity": cedar.String(f.PR.AuthorIdentity),
			"labels":         sketchV0StringSet(f.PR.Labels),
			"draft":          cedar.Boolean(f.PR.Draft),
		}
		if f.PR.Author != "" {
			pr["author"] = sketchV0PrincipalUID(f.PR.Author)
		}
		if f.PR.MergedByIdentity != "" {
			pr["mergedByIdentity"] = cedar.String(f.PR.MergedByIdentity)
		}
		ctx["pr"] = cedar.NewRecord(pr)
	}

	return cedar.Request{
		Principal: change,
		Action:    cedar.NewEntityUID("Gittuf::Action", cedar.String(f.Action)),
		Resource:  target,
		Context:   cedar.NewRecord(ctx),
	}, entities
}

// sketchV0CedarAllows is the fail-closed wrapper: Allow only if cedar-go says Allow
// AND reported no evaluation errors.
func sketchV0CedarAllows(ps *cedar.PolicySet, entities cedar.EntityMap, req cedar.Request) (bool, cedar.Decision, cedar.Diagnostic) {
	decision, diag := cedar.Authorize(ps, entities, req)
	return decision == cedar.Allow && len(diag.Errors) == 0, decision, diag
}

func sketchV0LoadCedarPolicySet(t testing.TB, files ...string) *cedar.PolicySet {
	t.Helper()
	ps := cedar.NewPolicySet()
	for _, f := range files {
		b, err := os.ReadFile(filepath.Join(sketchV0CedarSketchDir(), f))
		require.Nil(t, err)
		fps, err := cedar.NewPolicySetFromBytes(f, b)
		require.Nil(t, err)
		for id, p := range fps.All() {
			ps.Add(cedar.PolicyID(f+"/"+string(id)), p)
		}
	}
	return ps
}

func sketchV0LoadCedarEntities(t testing.TB) cedar.EntityMap {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(sketchV0CedarSketchDir(), "entities.json"))
	require.Nil(t, err)
	var ents cedar.EntityMap
	require.Nil(t, json.Unmarshal(b, &ents))
	return ents
}

func sketchV0Reasons(diag cedar.Diagnostic) string {
	out := []string{}
	for _, r := range diag.Reasons {
		out = append(out, string(r.PolicyID))
	}
	for _, e := range diag.Errors {
		out = append(out, "ERROR "+string(e.PolicyID)+": "+e.Message)
	}
	return strings.Join(out, "; ")
}

var sketchV0DemoPolicyFiles = []string{
	"01-forbid-principal-on-main.cedar",
	"02-require-group-member.cedar",
	"03-no-self-approval.cedar",
	"04-require-label.cedar",
}

func TestCedarSketchPolicies(t *testing.T) {
	t.Run("validate against schema (strict)", func(t *testing.T) {
		schemaBytes, err := os.ReadFile(filepath.Join(sketchV0CedarSketchDir(), "gittuf.cedarschema"))
		require.Nil(t, err)
		var s cedarschema.Schema
		s.SetFilename("gittuf.cedarschema")
		require.Nil(t, s.UnmarshalCedar(schemaBytes))
		rs, err := s.Resolve()
		require.Nil(t, err)
		v := cedarvalidate.New(rs, cedarvalidate.WithStrict())

		for _, f := range append([]string{"00-baseline.cedar"}, sketchV0DemoPolicyFiles...) {
			ps := sketchV0LoadCedarPolicySet(t, f)
			for id, p := range ps.All() {
				err := v.Policy(string(id), (*cedarxast.Policy)(p.AST()))
				t.Logf("validate %-60s err=%v", id, err)
				require.Nil(t, err)
			}
		}

		unsafe := sketchV0LoadCedarPolicySet(t, "99-unsafe-unguarded.cedar.txt")
		for id, p := range unsafe.All() {
			err := v.Policy(string(id), (*cedarxast.Policy)(p.AST()))
			t.Logf("validate %-60s err=%v", id, err)
			require.NotNil(t, err, "strict validation should reject unguarded optional attribute access")
		}

		require.Nil(t, v.Entities(sketchV0LoadCedarEntities(t)))

		req, ents := sketchV0BuildCedarRequest(sketchV0CedarFacts{Change: "e1", Action: "updateRef", Scheme: "git", Name: "refs/heads/main",
			Signers: []string{"R"}, PR: &sketchV0CedarPR{Number: 1, AuthorIdentity: "jane+1", Labels: []string{}}}, sketchV0LoadCedarEntities(t))
		require.Nil(t, v.Request(req))
		require.Nil(t, v.Entities(ents))
	})

	base := sketchV0LoadCedarEntities(t)
	labelled := &sketchV0CedarPR{Number: 7, AuthorIdentity: "jane.doe+1", Author: "jane.doe", Labels: []string{"security-reviewed"}}

	type scenario struct {
		name      string
		ruleFile  string
		violating sketchV0CedarFacts
		compliant sketchV0CedarFacts
	}
	mainFacts := func(signers, approvers, approverIdentities []string, pr *sketchV0CedarPR) sketchV0CedarFacts {
		return sketchV0CedarFacts{Change: "rsl-entry", Action: "updateRef", Scheme: "git", Name: "refs/heads/main",
			Signers: signers, Approvers: approvers, ApproverIdentities: approverIdentities,
			MatchedRules: []string{"protect-main"}, VerifiedUsing: "protect-main", PR: pr}
	}
	scenarios := []scenario{
		{
			name:      "rule 1: mallory credited on main",
			ruleFile:  "01-forbid-principal-on-main.cedar",
			violating: mainFacts([]string{"mallory"}, nil, nil, labelled),
			compliant: mainFacts([]string{"alice"}, nil, nil, labelled),
		},
		{
			name:      "rule 2: 3 signers, none a release manager",
			ruleFile:  "02-require-group-member.cedar",
			violating: mainFacts([]string{"a", "b", "c"}, nil, nil, labelled),
			compliant: mainFacts([]string{"R", "b", "c"}, nil, nil, labelled),
		},
		{
			name:      "rule 3: author approved own PR",
			ruleFile:  "03-no-self-approval.cedar",
			violating: mainFacts([]string{"john.doe"}, []string{"jane.doe"}, []string{"jane.doe+1"}, labelled),
			compliant: mainFacts([]string{"jane.doe"}, []string{"john.doe"}, []string{"john.doe+2"}, labelled),
		},
		{
			name:      "rule 3b: author unmapped, identity match only",
			ruleFile:  "03-no-self-approval.cedar",
			violating: mainFacts([]string{"john.doe"}, nil, []string{"ghost+9"}, &sketchV0CedarPR{Number: 8, AuthorIdentity: "ghost+9", Labels: []string{"security-reviewed"}}),
			compliant: mainFacts([]string{"john.doe"}, nil, []string{"john.doe+2"}, &sketchV0CedarPR{Number: 8, AuthorIdentity: "ghost+9", Labels: []string{"security-reviewed"}}),
		},
		{
			name:      "rule 4: label missing",
			ruleFile:  "04-require-label.cedar",
			violating: mainFacts([]string{"jane.doe"}, []string{"john.doe"}, []string{"john.doe+2"}, &sketchV0CedarPR{Number: 9, AuthorIdentity: "jane.doe+1", Labels: []string{"docs"}}),
			compliant: mainFacts([]string{"jane.doe"}, []string{"john.doe"}, []string{"john.doe+2"}, labelled),
		},
		{
			name:      "rule 4: no PR attestation at all (fails closed)",
			ruleFile:  "04-require-label.cedar",
			violating: mainFacts([]string{"jane.doe"}, nil, nil, nil),
			compliant: mainFacts([]string{"jane.doe"}, nil, nil, labelled),
		},
	}

	baseline := sketchV0LoadCedarPolicySet(t, "00-baseline.cedar")
	for _, sc := range scenarios {
		t.Run(sc.name, func(t *testing.T) {
			withRule := sketchV0LoadCedarPolicySet(t, "00-baseline.cedar", sc.ruleFile)

			req, ents := sketchV0BuildCedarRequest(sc.violating, base)
			ok, _, diag := sketchV0CedarAllows(baseline, ents, req)
			t.Logf("violating facts, permits-only:  allow=%t [%s]", ok, sketchV0Reasons(diag))
			require.True(t, ok)
			ok, _, diag = sketchV0CedarAllows(withRule, ents, req)
			t.Logf("violating facts, with rule:     allow=%t [%s]", ok, sketchV0Reasons(diag))
			require.False(t, ok)
			require.Empty(t, diag.Errors)

			req, ents = sketchV0BuildCedarRequest(sc.compliant, base)
			ok, _, diag = sketchV0CedarAllows(withRule, ents, req)
			t.Logf("compliant facts, with rule:     allow=%t [%s]", ok, sketchV0Reasons(diag))
			require.True(t, ok)
		})
	}

	t.Run("all four rules together on compliant facts", func(t *testing.T) {
		all := sketchV0LoadCedarPolicySet(t, append([]string{"00-baseline.cedar"}, sketchV0DemoPolicyFiles...)...)
		req, ents := sketchV0BuildCedarRequest(mainFacts([]string{"R", "jane.doe"}, []string{"john.doe"}, []string{"john.doe+2"}, labelled), base)
		ok, _, diag := sketchV0CedarAllows(all, ents, req)
		t.Logf("allow=%t [%s]", ok, sketchV0Reasons(diag))
		require.True(t, ok)
	})

	t.Run("unguarded forbid fails OPEN in cedar-go; wrapper fails closed", func(t *testing.T) {
		unsafe := sketchV0LoadCedarPolicySet(t, "00-baseline.cedar", "99-unsafe-unguarded.cedar.txt")
		req, ents := sketchV0BuildCedarRequest(mainFacts([]string{"jane.doe"}, nil, nil, nil), base)
		ok, decision, diag := sketchV0CedarAllows(unsafe, ents, req)
		t.Logf("raw cedar decision=%v, wrapper allow=%t [%s]", decision, ok, sketchV0Reasons(diag))
		require.Equal(t, cedar.Allow, decision, "cedar-go skips the erroring forbid")
		require.NotEmpty(t, diag.Errors)
		require.False(t, ok)
	})

	t.Run("empty policy set denies (default deny)", func(t *testing.T) {
		req, ents := sketchV0BuildCedarRequest(mainFacts([]string{"jane.doe"}, nil, nil, labelled), base)
		ok, decision, _ := sketchV0CedarAllows(cedar.NewPolicySet(), ents, req)
		t.Logf("decision=%v allow=%t", decision, ok)
		require.False(t, ok)
	})
}
