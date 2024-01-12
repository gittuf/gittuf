// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestInitializeRootMetadata(t *testing.T) {
	key, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)
	assert.Equal(t, 1, rootMetadata.Version)
	assert.Equal(t, key, rootMetadata.Keys[key.KeyID])
	assert.Equal(t, 1, rootMetadata.Roles[RootRoleName].Threshold)
	assert.Equal(t, []string{key.KeyID}, rootMetadata.Roles[RootRoleName].KeyIDs)
}

func TestAddRootKey(t *testing.T) {
	key, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

	newRootKey, err := tuf.LoadKeyFromBytes(targets1KeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata = AddRootKey(rootMetadata, newRootKey)

	assert.Equal(t, newRootKey, rootMetadata.Keys[newRootKey.KeyID])
	assert.Equal(t, []string{key.KeyID, newRootKey.KeyID}, rootMetadata.Roles[RootRoleName].KeyIDs)
}

func TestRemoveRootKey(t *testing.T) {
	key, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

	newRootKey, err := tuf.LoadKeyFromBytes(targets1KeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata = AddRootKey(rootMetadata, newRootKey)

	rootMetadata, err = DeleteRootKey(rootMetadata, newRootKey.KeyID)

	assert.Nil(t, err)
	assert.Equal(t, key, rootMetadata.Keys[key.KeyID])
	assert.Equal(t, newRootKey, rootMetadata.Keys[newRootKey.KeyID])
	assert.Equal(t, []string{key.KeyID}, rootMetadata.Roles[RootRoleName].KeyIDs)

	rootMetadata, err = DeleteRootKey(rootMetadata, key.KeyID)

	assert.ErrorIs(t, err, ErrCannotMeetThreshold)
	assert.Nil(t, rootMetadata)
}

func TestAddTargetsKey(t *testing.T) {
	key, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

	targetsKey, err := tuf.LoadKeyFromBytes(targets1KeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata = AddTargetsKey(rootMetadata, targetsKey)
	assert.Equal(t, targetsKey, rootMetadata.Keys[targetsKey.KeyID])
	assert.Equal(t, []string{targetsKey.KeyID}, rootMetadata.Roles[TargetsRoleName].KeyIDs)
}

func TestDeleteTargetsKey(t *testing.T) {
	key, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

	targetsKey1, err := tuf.LoadKeyFromBytes(targets1KeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	targetsKey2, err := tuf.LoadKeyFromBytes(targets2KeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata = AddTargetsKey(rootMetadata, targetsKey1)
	rootMetadata = AddTargetsKey(rootMetadata, targetsKey2)

	rootMetadata, err = DeleteTargetsKey(rootMetadata, targetsKey1.KeyID)
	assert.Nil(t, err)
	assert.Equal(t, targetsKey1, rootMetadata.Keys[targetsKey1.KeyID])
	assert.Equal(t, targetsKey2, rootMetadata.Keys[targetsKey2.KeyID])
	targetsRole := rootMetadata.Roles[TargetsRoleName]
	assert.Contains(t, targetsRole.KeyIDs, targetsKey2.KeyID)

	rootMetadata, err = DeleteTargetsKey(rootMetadata, targetsKey2.KeyID)
	assert.ErrorIs(t, err, ErrCannotMeetThreshold)
	assert.Nil(t, rootMetadata)
}
