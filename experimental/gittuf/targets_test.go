// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"testing"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/assert"
)

func TestInitializeTargets(t *testing.T) {
	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

	targetsKey := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

	t.Run("successful initialization", func(t *testing.T) {
		// The helper also runs InitializeTargets for this test
		r := createTestRepositoryWithRoot(t, "")

		if err := r.AddTopLevelTargetsKey(testCtx, rootSigner, targetsKey, false); err != nil {
			t.Fatal(err)
		}

		if err := r.InitializeTargets(testCtx, targetsSigner, policy.TargetsRoleName, false); err != nil {
			t.Fatal(err)
		}

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
		assert.Nil(t, err)
		assert.Contains(t, targetsMetadata.GetRules(), tufv01.AllowRule())
	})

	t.Run("invalid role name", func(t *testing.T) {
		r := createTestRepositoryWithRoot(t, "")

		if err := r.AddTopLevelTargetsKey(testCtx, rootSigner, targetsKey, false); err != nil {
			t.Fatal(err)
		}

		err := r.InitializeTargets(testCtx, targetsSigner, policy.RootRoleName, false)
		assert.ErrorIs(t, err, ErrInvalidPolicyName)
	})
}

func TestAddDelegation(t *testing.T) {
	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

	t.Run("valid rule / delegation name", func(t *testing.T) {
		r := createTestRepositoryWithPolicy(t, "")

		targetsPubKey := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

		ruleName := "test-rule"
		authorizedKeys := []tuf.Principal{targetsPubKey}
		rulePatterns := []string{"git:branch=main"}

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(targetsMetadata.GetPrincipals()))
		assert.Equal(t, 2, len(targetsMetadata.GetRules()))
		assert.Contains(t, targetsMetadata.GetRules(), tufv01.AllowRule())

		if err := r.AddPrincipalToTargets(testCtx, targetsSigner, policy.TargetsRoleName, authorizedKeys, false); err != nil {
			t.Fatal(err)
		}

		err = r.AddDelegation(testCtx, targetsSigner, policy.TargetsRoleName, ruleName, []string{targetsPubKey.KeyID}, rulePatterns, 1, false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err = state.GetTargetsMetadata(policy.TargetsRoleName, false)
		assert.Nil(t, err)
		assert.Contains(t, targetsMetadata.GetPrincipals(), targetsPubKey.ID())
		assert.Contains(t, targetsMetadata.GetPrincipals(), gpgKey.KeyID)
		assert.Equal(t, 2, len(targetsMetadata.GetPrincipals()))
		assert.Equal(t, 3, len(targetsMetadata.GetRules()))
		assert.Contains(t, targetsMetadata.GetRules(), &tufv01.Delegation{
			Name:        ruleName,
			Paths:       rulePatterns,
			Terminating: false,
			Role:        tufv01.Role{KeyIDs: set.NewSetFromItems(targetsPubKey.KeyID), Threshold: 1},
		})
		assert.Contains(t, targetsMetadata.GetRules(), tufv01.AllowRule())
	})

	t.Run("invalid rule name", func(t *testing.T) {
		r := createTestRepositoryWithPolicy(t, "")

		err := r.AddDelegation(testCtx, targetsSigner, policy.TargetsRoleName, policy.RootRoleName, nil, nil, 1, false)
		assert.ErrorIs(t, err, ErrInvalidPolicyName)
	})
}

func TestUpdateDelegation(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)
	targetsKey := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

	if err := r.AddPrincipalToTargets(testCtx, targetsSigner, policy.TargetsRoleName, []tuf.Principal{gpgKey, targetsKey}, false); err != nil {
		t.Fatal(err)
	}

	err = r.UpdateDelegation(testCtx, targetsSigner, policy.TargetsRoleName, "protect-main", []string{gpgKey.KeyID, targetsKey.KeyID}, []string{"git:refs/heads/main"}, 1, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(targetsMetadata.GetRules()))
	assert.Contains(t, targetsMetadata.GetRules(), &tufv01.Delegation{
		Name:        "protect-main",
		Paths:       []string{"git:refs/heads/main"},
		Terminating: false,
		Role:        tufv01.Role{KeyIDs: set.NewSetFromItems(gpgKey.KeyID, targetsKey.KeyID), Threshold: 1},
	})
}

func TestReorderDelegations(t *testing.T) {
	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
	targetsKey := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

	r := createTestRepositoryWithPolicy(t, "")

	if err := r.AddPrincipalToTargets(testCtx, targetsSigner, policy.TargetsRoleName, []tuf.Principal{targetsKey}, false); err != nil {
		t.Fatal(err)
	}

	ruleNames := []string{"rule-1", "rule-2", "rule-3"}
	for _, ruleName := range ruleNames {
		err := r.AddDelegation(testCtx, targetsSigner, policy.TargetsRoleName, ruleName, []string{targetsKey.KeyID}, []string{ruleName}, 1, false)
		if err != nil {
			t.Fatal(err)
		}
	}

	// Valid Input
	newOrder := []string{"rule-3", "rule-1", "rule-2", "protect-main"}
	err := r.ReorderDelegations(testCtx, targetsSigner, policy.TargetsRoleName, newOrder, false)
	if err != nil {
		t.Fatal(err)
	}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}
	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}

	finalOrder := []string{}
	for _, role := range targetsMetadata.GetRules() {
		finalOrder = append(finalOrder, role.ID())
	}
	expectedFinalOrder := append([]string{}, newOrder...)
	expectedFinalOrder = append(expectedFinalOrder, tuf.AllowRuleName)
	assert.Equal(t, expectedFinalOrder, finalOrder)
}

func TestRemoveDelegation(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
	targetsPubKey := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

	ruleName := "test-rule"
	authorizedKeys := []tuf.Principal{targetsPubKey}
	rulePatterns := []string{"git:branch=main"}

	if err := r.AddPrincipalToTargets(testCtx, targetsSigner, policy.TargetsRoleName, authorizedKeys, false); err != nil {
		t.Fatal(err)
	}

	err := r.AddDelegation(testCtx, targetsSigner, policy.TargetsRoleName, ruleName, []string{targetsPubKey.KeyID}, rulePatterns, 1, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.GetPrincipals(), targetsPubKey.ID())
	assert.Equal(t, 3, len(targetsMetadata.GetRules()))
	assert.Contains(t, targetsMetadata.GetRules(), &tufv01.Delegation{
		Name:        ruleName,
		Paths:       rulePatterns,
		Terminating: false,
		Role:        tufv01.Role{KeyIDs: set.NewSetFromItems(targetsPubKey.KeyID), Threshold: 1},
	})
	assert.Contains(t, targetsMetadata.GetRules(), tufv01.AllowRule())

	err = r.RemoveDelegation(testCtx, targetsSigner, policy.TargetsRoleName, ruleName, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = state.GetTargetsMetadata(policy.TargetsRoleName, false)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.GetPrincipals(), targetsPubKey.ID())
	assert.Equal(t, 2, len(targetsMetadata.GetRules()))
	assert.Contains(t, targetsMetadata.GetRules(), tufv01.AllowRule())
}

func TestAddPrincipalToTargets(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
	targetsPubKey := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	authorizedKeysBytes := []tuf.Principal{targetsPubKey, gpgKey}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.GetPrincipals(), gpgKey.KeyID)
	assert.Equal(t, 1, len(targetsMetadata.GetPrincipals()))

	err = r.AddPrincipalToTargets(testCtx, targetsSigner, policy.TargetsRoleName, authorizedKeysBytes, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = state.GetTargetsMetadata(policy.TargetsRoleName, false)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(targetsMetadata.GetPrincipals()))
}

func TestSignTargets(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	// Add root key as a targets key
	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootPubKey := tufv01.NewKeyFromSSLibKey(rootSigner.MetadataKey())

	if err := r.AddTopLevelTargetsKey(testCtx, rootSigner, rootPubKey, false); err != nil {
		t.Fatal(err)
	}

	// Add signature to targets
	err := r.SignTargets(testCtx, rootSigner, policy.TargetsRoleName, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(state.TargetsEnvelope.Signatures))
}
