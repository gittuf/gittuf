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

	keyID, err := key.ID()
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err := InitializeRootMetadata(key)
	assert.Nil(t, err)
	assert.Equal(t, 1, rootMetadata.Version)
	assert.Equal(t, *key, rootMetadata.Keys[keyID])
	assert.Equal(t, 1, rootMetadata.Roles[RootRoleName].Threshold)
	assert.Equal(t, []string{keyID}, rootMetadata.Roles[RootRoleName].KeyIDs)
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

	rootMetadata, err := InitializeRootMetadata(key)
	if err != nil {
		t.Fatal(err)
	}

	targetsKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets-1.pub"))
	if err != nil {
		t.Fatal(err)
	}

	targetsKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	targetsKeyID, err := targetsKey.ID()
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = AddTargetsKey(rootMetadata, targetsKey)
	assert.Nil(t, err)
	assert.Equal(t, *targetsKey, rootMetadata.Keys[targetsKeyID])
	assert.Equal(t, []string{targetsKeyID}, rootMetadata.Roles[TargetsRoleName].KeyIDs)
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

	rootMetadata, err := InitializeRootMetadata(key)
	if err != nil {
		t.Fatal(err)
	}

	targetsKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets-1.pub"))
	if err != nil {
		t.Fatal(err)
	}

	targetsKey1, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	targetsKey1ID, err := targetsKey1.ID()
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
	targetsKey2ID, err := targetsKey2.ID()
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = AddTargetsKey(rootMetadata, targetsKey1)
	if err != nil {
		t.Fatal(err)
	}
	rootMetadata, err = AddTargetsKey(rootMetadata, targetsKey2)
	if err != nil {
		t.Fatal(err)
	}

	rootMetadata, err = DeleteTargetsKey(rootMetadata, targetsKey1ID)
	assert.Nil(t, err)
	assert.Equal(t, *targetsKey1, rootMetadata.Keys[targetsKey1ID])
	assert.Equal(t, *targetsKey2, rootMetadata.Keys[targetsKey2ID])
	targetsRole := rootMetadata.Roles[TargetsRoleName]
	assert.Contains(t, targetsRole.KeyIDs, targetsKey2ID)

	rootMetadata, err = DeleteTargetsKey(rootMetadata, targetsKey2ID)
	assert.ErrorIs(t, err, ErrCannotMeetThreshold)
	assert.Nil(t, rootMetadata)
}
