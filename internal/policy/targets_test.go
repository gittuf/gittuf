package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestInitializeTargetsMetadata(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	assert.Equal(t, 1, targetsMetadata.Version)
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
}

func TestAddOrUpdateDelegation(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	keyBytes, err := os.ReadFile(filepath.Join("test-data", "targets-1.pub"))
	if err != nil {
		t.Fatal(err)
	}
	key1, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	key1ID, err := key1.ID()
	if err != nil {
		t.Fatal(err)
	}
	keyBytes, err = os.ReadFile(filepath.Join("test-data", "targets-2.pub"))
	if err != nil {
		t.Fatal(err)
	}
	key2, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	key2ID, err := key2.ID()
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "test-rule", []*tuf.Key{key1, key2}, []string{"test/"})
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, key1ID)
	assert.Equal(t, targetsMetadata.Delegations.Keys[key1ID], *key1)
	assert.Contains(t, targetsMetadata.Delegations.Keys, key2ID)
	assert.Equal(t, targetsMetadata.Delegations.Keys[key2ID], *key2)
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Equal(t, targetsMetadata.Delegations.Roles[0], tuf.Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        tuf.Role{KeyIDs: []string{key1ID, key2ID}, Threshold: 1},
	})
}

func TestRemoveDelegation(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	keyBytes, err := os.ReadFile(filepath.Join("test-data", "targets-1.pub"))
	if err != nil {
		t.Fatal(err)
	}
	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}
	keyID, err := key.ID()
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "test-rule", []*tuf.Key{key}, []string{"test/"})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))

	targetsMetadata, err = RemoveDelegation(targetsMetadata, "test-rule")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Contains(t, targetsMetadata.Delegations.Keys, keyID)
}

func TestAllowRule(t *testing.T) {
	allowRule := AllowRule()
	assert.Equal(t, AllowRuleName, allowRule.Name)
	assert.Equal(t, []string{"*"}, allowRule.Paths)
	assert.True(t, allowRule.Terminating)
	assert.Empty(t, allowRule.KeyIDs)
	assert.Equal(t, 1, allowRule.Threshold)
}
