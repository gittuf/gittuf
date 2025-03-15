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
}

func TestGetObjectSize(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	blobID, err := repo.WriteBlob([]byte("gittuf"))
	require.Nil(t, err)

	objSize, err := repo.GetObjectSize(blobID)
	assert.Nil(t, err)
	assert.Equal(t, uint64(6), objSize)
}
