// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"crypto"
	"crypto/ed25519"
	"errors"
	"slices"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/policy"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// shapeKeysForTest are the keys a policy shape refers to by label.
type shapeKeysForTest struct {
	priv map[string]ed25519.PrivateKey
	id   map[string]string
}

// policyShapeForTest describes a policy beyond what AuthorPolicy authors.
type policyShapeForTest struct {
	rules       []ruleForTest
	globalRules []tuf.GlobalRule
	delegations map[string][]ruleForTest
}

type ruleForTest struct {
	name        string
	keys        []string
	patterns    []string
	threshold   int
	lastTrusted bool
}

func newShapeKeysForTest(t *testing.T, labels ...string) shapeKeysForTest {
	t.Helper()

	keys := shapeKeysForTest{priv: map[string]ed25519.PrivateKey{}, id: map[string]string{}}
	for _, label := range labels {
		pub, priv := newTestED25519Key(t)
		keys.priv[label] = priv
		keys.id[label] = principalIDForTest(t, pub)
	}

	return keys
}

func ruleFileForTest(t *testing.T, keys shapeKeysForTest, rules []ruleForTest, expires time.Time) *tufv02.TargetsMetadata {
	t.Helper()

	metadata := tufv02.NewTargetsMetadata()
	metadata.SetExpires(expires.Format(time.RFC3339))
	for _, rule := range rules {
		ids := make([]string, 0, len(rule.keys))
		for _, label := range rule.keys {
			principal, err := newPrincipal(keys.priv[label].Public())
			require.NoError(t, err)
			if _, has := metadata.Delegations.Principals[principal.ID()]; !has {
				require.NoError(t, metadata.AddPrincipal(principal))
			}
			ids = append(ids, principal.ID())
		}
		require.NoError(t, metadata.AddRule(rule.name, ids, rule.patterns, rule.threshold))
		for _, role := range metadata.Delegations.Roles {
			if role.Name == rule.name {
				role.Terminating = rule.lastTrusted
			}
		}
	}

	return metadata
}

// applyPolicyShapeForTest signs and applies shape. Each delegated rule file is
// signed by the keys of the rule delegating to it.
func applyPolicyShapeForTest(t *testing.T, store *gitinterface.Repository, keys shapeKeysForTest, shape policyShapeForTest) {
	t.Helper()

	params, _ := authorPolicyParamsForTest(t)
	rootSigners, err := signersFromCryptoSigners(params.RootSigners)
	require.NoError(t, err)
	rootPrincipals, err := principalsFromPublicKeys(params.RootPublicKeys)
	require.NoError(t, err)

	rootMetadata, err := params.rootMetadata(rootPrincipals)
	require.NoError(t, err)
	for _, globalRule := range shape.globalRules {
		require.NoError(t, rootMetadata.AddGlobalRule(globalRule))
	}
	rootEnv, err := signMetadata(t.Context(), rootMetadata, rootSigners)
	require.NoError(t, err)

	targetsEnv, err := signMetadata(t.Context(), ruleFileForTest(t, keys, shape.rules, params.Expires), rootSigners)
	require.NoError(t, err)

	delegationEnvs := map[string]*sslibdsse.Envelope{}
	for name, rules := range shape.delegations {
		var delegators []crypto.Signer
		for _, rule := range shape.rules {
			if rule.name == name {
				for _, label := range rule.keys {
					delegators = append(delegators, keys.priv[label])
				}
			}
		}
		signers, err := signersFromCryptoSigners(delegators)
		require.NoError(t, err)

		delegationEnvs[name], err = signMetadata(t.Context(), ruleFileForTest(t, keys, rules, params.Expires), signers)
		require.NoError(t, err)
	}

	state := &policy.State{
		Metadata: &policy.StateMetadata{
			RootEnvelope:        rootEnv,
			TargetsEnvelope:     targetsEnv,
			DelegationEnvelopes: delegationEnvs,
		},
	}
	require.NoError(t, state.Verify(t.Context()))

	tree, err := state.WriteTree(store)
	require.NoError(t, err)
	applyPolicyTreeForTest(t, store, tree)
}

// Satisfies must agree with verification. Each shape is applied in a fresh
// repository per key, an RSL entry for authorTestRef is signed with that key,
// and VerifyRef must pass exactly when Satisfies reports the key alone meets
// the policy or that nothing protects the ref.
func TestPolicySatisfiesAgreesWithVerification(t *testing.T) {
	t.Parallel()

	main := []string{"git:" + authorTestRef}
	branches := []string{"git:refs/heads/*"}

	tests := map[string]struct {
		shape policyShapeForTest
		met   []string
	}{
		"threshold two": {
			shape: policyShapeForTest{rules: []ruleForTest{{name: "main", keys: []string{"a", "b"}, patterns: main, threshold: 2}}},
		},
		"rule for another ref": {
			shape: policyShapeForTest{rules: []ruleForTest{
				{name: "release", keys: []string{"a"}, patterns: []string{"git:refs/heads/release-*"}, threshold: 1},
				{name: "main", keys: []string{"b"}, patterns: main, threshold: 1},
			}},
			met: []string{"b"},
		},
		"delegated rule file": {
			shape: policyShapeForTest{
				rules:       []ruleForTest{{name: "team", keys: []string{"a"}, patterns: branches, threshold: 1}},
				delegations: map[string][]ruleForTest{"team": {{name: "team-main", keys: []string{"x"}, patterns: main, threshold: 1}}},
			},
			met: []string{"a", "x"},
		},
		"rule after last trusted delegation": {
			shape: policyShapeForTest{
				rules: []ruleForTest{
					{name: "team", keys: []string{"a"}, patterns: branches, threshold: 1, lastTrusted: true},
					{name: "fallback", keys: []string{"b"}, patterns: branches, threshold: 1},
				},
				delegations: map[string][]ruleForTest{"team": {{name: "team-main", keys: []string{"x"}, patterns: main, threshold: 1}}},
			},
			met: []string{"a", "x"},
		},
		"global threshold rule": {
			shape: policyShapeForTest{
				rules:       []ruleForTest{{name: "main", keys: []string{"a"}, patterns: main, threshold: 1}},
				globalRules: []tuf.GlobalRule{tufv02.NewGlobalRuleThreshold("two-for-main", main, 2)},
			},
		},
		"unprotected ref": {
			shape: policyShapeForTest{rules: []ruleForTest{{name: "other", keys: []string{"a"}, patterns: []string{"git:refs/heads/other"}, threshold: 1}}},
			met:   []string{"a", "b", "x"},
		},
	}

	for name, test := range tests {
		for _, signer := range []string{"a", "b", "x"} {
			t.Run(name+"/"+signer, func(t *testing.T) {
				t.Parallel()

				store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
				keys := newShapeKeysForTest(t, "a", "b", "x")
				applyPolicyShapeForTest(t, store, keys, test.shape)
				recordRefUpdateForTest(t, store, keys.priv[signer])

				loaded, err := NewStorerRepository(store).LoadPolicy(t.Context())
				require.NoError(t, err)
				_, satisfiesErr := loaded.Satisfies(authorTestRef, keys.id[signer])
				_, verifyErr := NewStorerRepository(store).VerifyRef(t.Context(), authorTestRef)

				want := slices.Contains(test.met, signer)
				assert.Equal(t, want, verifyErr == nil, "verification: %v", verifyErr)
				assert.Equal(t, want, satisfiesErr == nil || errors.Is(satisfiesErr, ErrRefNotProtected), "satisfies: %v", satisfiesErr)
			})
		}
	}
}

func TestPolicyPrincipalsIncludeDelegatedRuleFiles(t *testing.T) {
	t.Parallel()

	store := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	keys := newShapeKeysForTest(t, "a", "x")
	applyPolicyShapeForTest(t, store, keys, policyShapeForTest{
		rules:       []ruleForTest{{name: "team", keys: []string{"a"}, patterns: []string{"git:refs/heads/*"}, threshold: 1}},
		delegations: map[string][]ruleForTest{"team": {{name: "team-main", keys: []string{"x"}, patterns: []string{"git:" + authorTestRef}, threshold: 1}}},
	})

	loaded, err := NewStorerRepository(store).LoadPolicy(t.Context())
	require.NoError(t, err)
	assert.Contains(t, loaded.Principals(), keys.id["x"])
}
