// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	_ "embed"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

//go:embed test-data/gpg-pubkey.asc
var gpgPubKeyBytes []byte

func TestInitializeTargets(t *testing.T) {
	// The helper also runs InitializeTargets for this test
	r, _ := createTestRepositoryWithTargets(t)

	state, err := policy.LoadCurrentState(context.Background(), r.r)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Equal(t, 1, targetsMetadata.Version)
	assert.Empty(t, targetsMetadata.Targets)
	assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())
}

func TestAddDelegation(t *testing.T) {
	r, targetsKeyBytes := createTestRepositoryWithTargets(t)

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

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName)
	assert.Nil(t, err)
	assert.Empty(t, targetsMetadata.Delegations.Keys)
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
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.Delegation{
		Name:        ruleName,
		Paths:       rulePatterns,
		Terminating: false,
		Role:        tuf.Role{KeyIDs: []string{targetsKey.KeyID}, Threshold: 1},
	})
	assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())
}

func TestRemoveDelegation(t *testing.T) {
	r, targetsKeyBytes := createTestRepositoryWithTargets(t)

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
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))
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
	assert.Equal(t, 1, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, policy.AllowRule())
}

func TestAddKeyToTargets(t *testing.T) {
	r, targetsKeyBytes := createTestRepositoryWithTargets(t)

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
	assert.Empty(t, targetsMetadata.Delegations.Keys)

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

func createTestRepositoryWithTargets(t *testing.T) (*Repository, []byte) {
	t.Helper()

	r, rootKeyBytes := createTestRepositoryWithRoot(t)

	targetsKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets"))
	if err != nil {
		t.Fatal(err)
	}

	if err := r.AddTopLevelTargetsKey(context.Background(), rootKeyBytes, targetsKeyBytes, false); err != nil {
		t.Fatal(err)
	}

	err = r.InitializeTargets(context.Background(), targetsKeyBytes, policy.TargetsRoleName, false)
	if err != nil {
		t.Fatal(err)
	}

	return r, targetsKeyBytes
}
