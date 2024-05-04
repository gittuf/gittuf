// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestInitializeTargets(t *testing.T) {
	targetsKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	targetsSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	t.Run("successful initialization", func(t *testing.T) {
		// The helper also runs InitializeTargets for this test
		r, _ := createTestRepositoryWithRoot(t, "")

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

		targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
		assert.Nil(t, err)
		assert.Empty(t, targetsMetadata.Targets)
		assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())
	})

	t.Run("invalid role name", func(t *testing.T) {
		r, _ := createTestRepositoryWithRoot(t, "")

		if err := r.AddTopLevelTargetsKey(testCtx, rootSigner, targetsKey, false); err != nil {
			t.Fatal(err)
		}

		err := r.InitializeTargets(testCtx, targetsSigner, policy.RootRoleName, false)
		assert.ErrorIs(t, err, ErrInvalidPolicyName)
	})
}

func TestAddDelegation(t *testing.T) {
	targetsSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	t.Run("valid rule / delegation name", func(t *testing.T) {
		r := createTestRepositoryWithPolicy(t, "")

		targetsPubKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		ruleName := "test-rule"
		authorizedKeyBytes := []*tuf.Key{targetsPubKey}
		rulePatterns := []string{"git:branch=main"}

		state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
		assert.Nil(t, err)
		assert.Equal(t, 1, len(targetsMetadata.Delegations.Keys))
		assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))
		assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())

		err = r.AddDelegation(testCtx, targetsSigner, policy.TargetsRoleName, ruleName, authorizedKeyBytes, rulePatterns, 1, false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
		if err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err = state.GetTargetsMetadata(policy.TargetsRoleName)
		assert.Nil(t, err)
		assert.Contains(t, targetsMetadata.Delegations.Keys, targetsPubKey.KeyID)
		assert.Contains(t, targetsMetadata.Delegations.Keys, gpgKey.KeyID)
		assert.Equal(t, 2, len(targetsMetadata.Delegations.Keys))
		assert.Equal(t, 3, len(targetsMetadata.Delegations.Roles))
		assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.Delegation{
			Name:        ruleName,
			Paths:       rulePatterns,
			Terminating: false,
			Role:        tuf.Role{KeyIDs: []string{targetsPubKey.KeyID}, Threshold: 1},
		})
		assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())
	})

	t.Run("invalid rule name", func(t *testing.T) {
		r := createTestRepositoryWithPolicy(t, "")

		err := r.AddDelegation(testCtx, targetsSigner, policy.TargetsRoleName, policy.RootRoleName, nil, nil, 1, false)
		assert.ErrorIs(t, err, ErrInvalidPolicyName)
	})
}

func TestUpdateDelegation(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	targetsSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	targetsKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	err = r.UpdateDelegation(testCtx, targetsSigner, policy.TargetsRoleName, "protect-main", []*tuf.Key{gpgKey, targetsKey}, []string{"git:refs/heads/main"}, 1, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.Delegation{
		Name:        "protect-main",
		Paths:       []string{"git:refs/heads/main"},
		Terminating: false,
		Role:        tuf.Role{KeyIDs: []string{gpgKey.KeyID, targetsKey.KeyID}, Threshold: 1},
	})
}

func TestRemoveDelegation(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	targetsSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	targetsPubKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	ruleName := "test-rule"
	authorizedKeyBytes := []*tuf.Key{targetsPubKey}
	rulePatterns := []string{"git:branch=main"}

	err = r.AddDelegation(testCtx, targetsSigner, policy.TargetsRoleName, ruleName, authorizedKeyBytes, rulePatterns, 1, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, targetsPubKey.KeyID)
	assert.Equal(t, 3, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.Delegation{
		Name:        ruleName,
		Paths:       rulePatterns,
		Terminating: false,
		Role:        tuf.Role{KeyIDs: []string{targetsPubKey.KeyID}, Threshold: 1},
	})
	assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())

	err = r.RemoveDelegation(testCtx, targetsSigner, policy.TargetsRoleName, ruleName, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, targetsPubKey.KeyID)
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())
}

func TestAddKeyToTargets(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	targetsSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	targetsPubKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	authorizedKeysBytes := []*tuf.Key{targetsPubKey, gpgKey}

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, gpgKey.KeyID)
	assert.Equal(t, 1, len(targetsMetadata.Delegations.Keys))

	err = r.AddKeyToTargets(testCtx, targetsSigner, policy.TargetsRoleName, authorizedKeysBytes, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Keys))
}

func TestSignTargets(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	// Add root key as a targets key
	rootSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}
	rootPubKey, err := tuf.LoadKeyFromBytes(rootPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	if err := r.AddTopLevelTargetsKey(testCtx, rootSigner, rootPubKey, false); err != nil {
		t.Fatal(err)
	}

	// Add signature to targets
	err = r.SignTargets(testCtx, rootSigner, policy.TargetsRoleName, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(testCtx, r.r, policy.PolicyStagingRef)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, 2, len(state.TargetsEnvelope.Signatures))
}
