// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryReadBlob(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	contents := []byte("test file read")
	expectedBlobID, err := NewHash("2ecdd330475d93568ed27f717a84a7fe207d1c58")
	require.Nil(t, err)

	t.Run("read existing blob", func(t *testing.T) {
		blobID, err := repo.WriteBlob(contents)
		require.Nil(t, err)
		assert.Equal(t, expectedBlobID, blobID)

		readContents, err := repo.ReadBlob(blobID)
		assert.Nil(t, err)
		assert.Equal(t, contents, readContents)
	})

	t.Run("read non-existing blob", func(t *testing.T) {
		_, err := repo.ReadBlob(ZeroHash)
		assert.NotNil(t, err)
	})

	t.Run("read non-blob object", func(t *testing.T) {
		// Create a tree object using WriteTree
		treeBuilder := NewTreeBuilder(repo)
		blobID, err := repo.WriteBlob([]byte("test"))
		require.Nil(t, err)
		
		treeID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("file.txt", blobID),
		})
		require.Nil(t, err)

		// Try to read tree as blob - should fail
		_, err = repo.ReadBlob(treeID)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "is not a blob object")
	})
}

func TestRepositoryWriteBlob(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	t.Run("write blob and verify hash", func(t *testing.T) {
		contents := []byte("test file write")
		expectedBlobID, err := NewHash("999c05e9578e5d244920306842f516789a2498f7")
		require.Nil(t, err)

		blobID, err := repo.WriteBlob(contents)
		assert.Nil(t, err)
		assert.Equal(t, expectedBlobID, blobID)
	})

	t.Run("write empty blob", func(t *testing.T) {
		emptyBlobID, err := NewHash("e69de29bb2d1d6434b8b29ae775ad8c2e48c5391")
		require.Nil(t, err)

		blobID, err := repo.WriteBlob([]byte{})
		assert.Nil(t, err)
		assert.Equal(t, emptyBlobID, blobID)
	})
}
