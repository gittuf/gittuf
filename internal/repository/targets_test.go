// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestInitializeTargets(t *testing.T) {
	// The helper also runs InitializeTargets for this test
	r := createTestRepositoryWithPolicy(t, "")

	state, err := policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Equal(t, 2, targetsMetadata.Version)
	assert.Empty(t, targetsMetadata.Targets)
	assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())
}

func TestAddDelegation(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	targetsKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	ruleName := "test-rule"
	authorizedKeyBytes := [][]byte{targetsKeyBytes}
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
	assert.Contains(t, targetsMetadata.Delegations.Keys, gpgKey.KeyID)
	assert.Equal(t, 1, len(targetsMetadata.Delegations.Keys))
	assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())

	err = r.AddDelegation(context.Background(), targetsKeyBytes, policy.TargetsRoleName, ruleName, authorizedKeyBytes, rulePatterns, false)
	assert.Nil(t, err)

	state, err = policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, targetsKey.KeyID)
	assert.Equal(t, 3, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.Delegation{
		Name:        ruleName,
		Paths:       rulePatterns,
		Terminating: false,
		Role:        tuf.Role{KeyIDs: []string{targetsKey.KeyID}, Threshold: 1},
	})
	assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())
}

func TestRemoveDelegation(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	targetsKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	ruleName := "test-rule"
	authorizedKeyBytes := [][]byte{targetsKeyBytes}
	rulePatterns := []string{"git:branch=main"}

	err = r.AddDelegation(context.Background(), targetsKeyBytes, policy.TargetsRoleName, ruleName, authorizedKeyBytes, rulePatterns, false)
	assert.Nil(t, err)

	state, err := policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, targetsKey.KeyID)
	assert.Equal(t, 3, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.Delegation{
		Name:        ruleName,
		Paths:       rulePatterns,
		Terminating: false,
		Role:        tuf.Role{KeyIDs: []string{targetsKey.KeyID}, Threshold: 1},
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
	assert.Contains(t, targetsMetadata.Delegations.Keys, targetsKey.KeyID)
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())
}

func TestAddKeyToTargets(t *testing.T) {
	r := createTestRepositoryWithPolicy(t, "")

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	kb, err := json.Marshal(gpgKey)
	if err != nil {
		t.Fatal(err)
	}

	authorizedKeysBytes := [][]byte{targetsKeyBytes, kb}

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
