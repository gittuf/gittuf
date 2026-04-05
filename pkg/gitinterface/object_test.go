// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHasObject(t *testing.T) {
	tempDir1 := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir1, true)

	// Create a backup repo to compute Git IDs we test in repo
	tempDir2 := t.TempDir()
	backupRepo := CreateTestGitRepository(t, tempDir2, true)

	blobID, err := backupRepo.WriteBlob([]byte("hello"))
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, backupRepo.HasObject(blobID)) // backup has it
	assert.False(t, repo.HasObject(blobID))      // repo does not

	if _, err := repo.WriteBlob([]byte("hello")); err != nil {
		t.Fatal(err)
	}

	assert.True(t, repo.HasObject(blobID)) // now repo has it too

	backupRepoTreeBuilder := NewTreeBuilder(backupRepo)
	treeID, err := backupRepoTreeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("file", blobID)})
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, backupRepo.HasObject(treeID)) // backup has it
	assert.False(t, repo.HasObject(treeID))      // repo does not

	repoTreeBuilder := NewTreeBuilder(repo)
	if _, err := repoTreeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("file", blobID)}); err != nil {
		t.Fatal(err)
	}

	assert.True(t, repo.HasObject(treeID)) // now repo has it too

	commitID, err := backupRepo.Commit(treeID, "refs/heads/main", "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, backupRepo.HasObject(commitID)) // backup has it
	assert.False(t, repo.HasObject(commitID))      // repo does not

	if _, err := repo.Commit(treeID, "refs/heads/main", "Initial commit\n", false); err != nil {
		t.Fatal(err)
	}

	// Note: This test passes because we control timestamps in
	// CreateTestGitRepository. So, commit ID in both repos is the same.
	assert.True(t, repo.HasObject(commitID)) // now repo has it too

	t.Run("zero hash does not exist", func(t *testing.T) {
		assert.False(t, repo.HasObject(ZeroHash))
	})

	t.Run("non-existent hash", func(t *testing.T) {
		nonExistentHash, err := NewHash("aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa")
		require.Nil(t, err)
		assert.False(t, repo.HasObject(nonExistentHash))
	})

	t.Run("different object types", func(t *testing.T) {
		blob2ID, err := repo.WriteBlob([]byte("test"))
		require.Nil(t, err)
		assert.True(t, repo.HasObject(blob2ID))

		treeBuilder := NewTreeBuilder(repo)
		tree2ID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("f.txt", blob2ID)})
		require.Nil(t, err)
		assert.True(t, repo.HasObject(tree2ID))

		commit2ID, err := repo.Commit(tree2ID, "refs/heads/test", "Test\n", false)
		require.Nil(t, err)
		assert.True(t, repo.HasObject(commit2ID))
	})
}

func TestGetObjectType(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	blobID, err := repo.WriteBlob([]byte("gittuf"))
	require.Nil(t, err)

	objType, err := repo.GetObjectType(blobID)
	assert.Nil(t, err)
	assert.Equal(t, BlobObjectType, objType)

	treeBuilder := NewTreeBuilder(repo)
	treeID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("foo", blobID)})
	require.Nil(t, err)

	objType, err = repo.GetObjectType(treeID)
	assert.Nil(t, err)
	assert.Equal(t, TreeObjectType, objType)

	commitID, err := repo.Commit(treeID, "refs/heads/main", "Test commit\n", false)
	require.Nil(t, err)

	objType, err = repo.GetObjectType(commitID)
	assert.Nil(t, err)
	assert.Equal(t, CommitObjectType, objType)

	tagID, err := repo.TagUsingSpecificKey(commitID, "test-tag", "Test tag\n", artifacts.GPGKey1Private)
	require.Nil(t, err)

	objType, err = repo.GetObjectType(tagID)
	assert.Nil(t, err)
	assert.Equal(t, TagObjectType, objType)

	t.Run("error with non-existent object", func(t *testing.T) {
		_, err := repo.GetObjectType(ZeroHash)
		assert.NotNil(t, err)
	})
}

func TestGetObjectSize(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	blobID, err := repo.WriteBlob([]byte("gittuf"))
	require.Nil(t, err)

	objSize, err := repo.GetObjectSize(blobID)
	assert.Nil(t, err)
	assert.Equal(t, uint64(6), objSize)

	t.Run("error with non-existent object", func(t *testing.T) {
		_, err := repo.GetObjectSize(ZeroHash)
		assert.NotNil(t, err)
	})

	t.Run("empty blob", func(t *testing.T) {
		emptyBlobID, err := repo.WriteBlob([]byte{})
		require.Nil(t, err)

		size, err := repo.GetObjectSize(emptyBlobID)
		assert.Nil(t, err)
		assert.Equal(t, uint64(0), size)
	})

	t.Run("various sizes", func(t *testing.T) {
		sizes := []int{1, 10, 100, 1000}
		for _, sz := range sizes {
			content := make([]byte, sz)
			for i := range content {
				content[i] = byte(i % 256)
			}

			blobID, err := repo.WriteBlob(content)
			require.Nil(t, err)

			objSize, err := repo.GetObjectSize(blobID)
			assert.Nil(t, err)
			assert.Equal(t, uint64(sz), objSize) //nolint:gosec // Test size comparison
		}
	})

	t.Run("different object types", func(t *testing.T) {
		emptyTreeID, err := repo.EmptyTree()
		require.Nil(t, err)

		size, err := repo.GetObjectSize(emptyTreeID)
		assert.Nil(t, err)
		assert.GreaterOrEqual(t, size, uint64(0))

		commitID, err := repo.Commit(emptyTreeID, "refs/heads/size-test", "Test\n", false)
		require.Nil(t, err)

		size, err = repo.GetObjectSize(commitID)
		assert.Nil(t, err)
		assert.Greater(t, size, uint64(0))
	})
}
