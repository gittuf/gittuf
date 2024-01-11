// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	_ "embed"
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestInitializeTargets(t *testing.T) {
	targetsKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("successful initialization", func(t *testing.T) {
		// The helper also runs InitializeTargets for this test
		r, _ := createTestRepositoryWithRoot(t, "")

		if err := r.AddTopLevelTargetsKey(testCtx, rootKeyBytes, targetsKey, false); err != nil {
			t.Fatal(err)
		}

		err := r.InitializeTargets(testCtx, targetsKeyBytes, policy.TargetsRoleName, false)
		if err != nil {
			t.Fatal(err)
		}

		state, err := policy.LoadCurrentState(context.Background(), r.r)
		if err != nil {
			t.Fatal(err)
		}

		targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
		assert.Nil(t, err)
		assert.Equal(t, 1, targetsMetadata.Version)
		assert.Empty(t, targetsMetadata.Targets)
		assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())
	})

	t.Run("invalid role name", func(t *testing.T) {
		r, _ := createTestRepositoryWithRoot(t, "")

		if err := r.AddTopLevelTargetsKey(testCtx, rootKeyBytes, targetsKey, false); err != nil {
			t.Fatal(err)
		}

		err := r.InitializeTargets(testCtx, targetsKeyBytes, policy.RootRoleName, false)
		assert.ErrorIs(t, err, ErrInvalidPolicyName)
	})
}

func TestAddDelegation(t *testing.T) {
	t.Run("valid rule / delegation name", func(t *testing.T) {
		r := createTestRepositoryWithPolicy(t, "")

		targetsPubKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		ruleName := "test-rule"
		authorizedKeyBytes := []*tuf.Key{targetsPubKey}
		rulePatterns := []string{"git:branch=main"}

		state, err := policy.LoadCurrentState(context.Background(), r.r)
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

		err = r.AddDelegation(context.Background(), targetsKeyBytes, policy.TargetsRoleName, ruleName, authorizedKeyBytes, rulePatterns, false)
		assert.Nil(t, err)

		state, err = policy.LoadCurrentState(context.Background(), r.r)
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

		err := r.AddDelegation(testCtx, targetsKeyBytes, policy.TargetsRoleName, policy.RootRoleName, nil, nil, false)
		assert.ErrorIs(t, err, ErrInvalidPolicyName)
	})
}

func TestRemoveDelegation(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	targetsPubKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	ruleName := "test-rule"
	authorizedKeyBytes := []*tuf.Key{targetsPubKey}
	rulePatterns := []string{"git:branch=main"}

	err = r.AddDelegation(context.Background(), targetsKeyBytes, policy.TargetsRoleName, ruleName, authorizedKeyBytes, rulePatterns, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(context.Background(), r.r)
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

	err = r.RemoveDelegation(context.Background(), targetsKeyBytes, policy.TargetsRoleName, ruleName, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(context.Background(), r.r)
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

	targetsPubKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	authorizedKeysBytes := []*tuf.Key{targetsPubKey, gpgKey}

	state, err := policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, gpgKey.KeyID)
	assert.Equal(t, 1, len(targetsMetadata.Delegations.Keys))

	err = r.AddKeyToTargets(context.Background(), targetsKeyBytes, policy.TargetsRoleName, authorizedKeysBytes, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Keys))
}
