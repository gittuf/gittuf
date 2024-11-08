// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/stretchr/testify/assert"
)

func TestListRules(t *testing.T) {
	t.Run("no delegations", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)

		rules, err := ListRules(context.Background(), repo, PolicyRef)
		assert.Nil(t, err)

		expectedRules := []*DelegationWithDepth{
			{
				Delegation: &tufv02.Delegation{
					Name:        "protect-main",
					Paths:       []string{"git:refs/heads/main"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("157507bbe151e378ce8126c1dcfe043cdd2db96e"),
						Threshold:    1,
					},
				},
				Depth: 0,
			},
			{
				Delegation: &tufv02.Delegation{
					Name:        "protect-files-1-and-2",
					Paths:       []string{"file:1", "file:2"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("157507bbe151e378ce8126c1dcfe043cdd2db96e"),
						Threshold:    1,
					},
				},
				Depth: 0,
			},
		}
		assert.Equal(t, expectedRules, rules)
	})

	t.Run("with delegations", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithDelegatedPolicies)

		rules, err := ListRules(context.Background(), repo, PolicyRef)
		assert.Nil(t, err)

		expectedRules := []*DelegationWithDepth{
			{
				Delegation: &tufv02.Delegation{
					Name:        "1",
					Paths:       []string{"file:1/*"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("SHA256:ESJezAOo+BsiEpddzRXS6+wtF16FID4NCd+3gj96rFo"),
						Threshold:    1,
					},
				},
				Depth: 0,
			},
			{
				Delegation: &tufv02.Delegation{
					Name:        "3",
					Paths:       []string{"file:1/subpath1/*"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("157507bbe151e378ce8126c1dcfe043cdd2db96e"),
						Threshold:    1,
					},
				},
				Depth: 1,
			},
			{
				Delegation: &tufv02.Delegation{
					Name:        "4",
					Paths:       []string{"file:1/subpath2/*"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("157507bbe151e378ce8126c1dcfe043cdd2db96e"),
						Threshold:    1,
					},
				},
				Depth: 1,
			},

			{
				Delegation: &tufv02.Delegation{
					Name:        "2",
					Paths:       []string{"file:2/*"},
					Terminating: false,
					Custom:      nil,
					Role: tufv02.Role{
						PrincipalIDs: set.NewSetFromItems("SHA256:ESJezAOo+BsiEpddzRXS6+wtF16FID4NCd+3gj96rFo"),
						Threshold:    1,
					},
				},
				Depth: 0,
			},
		}
		assert.Equal(t, expectedRules, rules)
	})
}

func TestListPrincipals(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	t.Run("policy exists", func(t *testing.T) {
		pubKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}
		pubKey := tufv02.NewKeyFromSSLibKey(pubKeyR)
		expectedPrincipals := map[string]tuf.Principal{pubKey.KeyID: pubKey}

		principals, err := ListPrincipals(context.Background(), repo, PolicyRef, tuf.TargetsRoleName)
		assert.Nil(t, err)
		assert.Equal(t, expectedPrincipals, principals)
	})

	t.Run("policy does not exist", func(t *testing.T) {
		principals, err := ListPrincipals(testCtx, repo, PolicyRef, "does-not-exist")
		assert.ErrorIs(t, err, ErrPolicyNotFound)
		assert.Nil(t, principals)
	})
}
