package policy

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestInitializeRootMetadata(t *testing.T) {
	keyBytes, err := os.ReadFile(filepath.Join("test-data", rootPublicKeysTreeEntryName, "437cdafde81f715cf81e75920d7d4a9ce4cab83aac5a8a5984c3902da6bf2ab7"))
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)
	assert.Equal(t, 1, rootMetadata.Version)
	assert.Equal(t, *key, rootMetadata.Keys[key.ID()])
	assert.Equal(t, 1, rootMetadata.Roles[RootRoleName].Threshold)
	assert.Equal(t, []string{key.ID()}, rootMetadata.Roles[RootRoleName].KeyIDs)
}

func TestAddTargetsKey(t *testing.T) {
	keyBytes, err := os.ReadFile(filepath.Join("test-data", rootPublicKeysTreeEntryName, "437cdafde81f715cf81e75920d7d4a9ce4cab83aac5a8a5984c3902da6bf2ab7"))
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

	targetsKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets-1.pub"))
	if err != nil {
		t.Fatal(err)
	}

	targetsKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata = AddTargetsKey(rootMetadata, targetsKey)
	assert.Equal(t, *targetsKey, rootMetadata.Keys[targetsKey.ID()])
	assert.Equal(t, []string{targetsKey.ID()}, rootMetadata.Roles[TargetsRoleName].KeyIDs)
}

func TestDeleteTargetsKey(t *testing.T) {
	keyBytes, err := os.ReadFile(filepath.Join("test-data", rootPublicKeysTreeEntryName, "437cdafde81f715cf81e75920d7d4a9ce4cab83aac5a8a5984c3902da6bf2ab7"))
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(keyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata := InitializeRootMetadata(key)

	targetsKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets-1.pub"))
	if err != nil {
		t.Fatal(err)
	}

	targetsKey1, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	targetsKeyBytes, err = os.ReadFile(filepath.Join("test-data", "targets-2.pub"))
	if err != nil {
		t.Fatal(err)
	}

	targetsKey2, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata = AddTargetsKey(rootMetadata, targetsKey1)
	rootMetadata = AddTargetsKey(rootMetadata, targetsKey2)

	rootMetadata, err = DeleteTargetsKey(rootMetadata, targetsKey1.ID())
	assert.Nil(t, err)
	assert.Equal(t, *targetsKey1, rootMetadata.Keys[targetsKey1.ID()])
	assert.Equal(t, *targetsKey2, rootMetadata.Keys[targetsKey2.ID()])
	targetsRole := rootMetadata.Roles[TargetsRoleName]
	assert.Contains(t, targetsRole.KeyIDs, targetsKey2.ID())

	rootMetadata, err = DeleteTargetsKey(rootMetadata, targetsKey2.ID())
	assert.ErrorIs(t, err, ErrCannotMeetThreshold)
	assert.Nil(t, rootMetadata)
}
