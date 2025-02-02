// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/stretchr/testify/assert"
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
	treeID, err := backupRepoTreeBuilder.WriteTreeFromEntryIDs(map[string]Hash{"file": blobID})
	if err != nil {
		t.Fatal(err)
	}

	assert.True(t, backupRepo.HasObject(treeID)) // backup has it
	assert.False(t, repo.HasObject(treeID))      // repo does not

	repoTreeBuilder := NewTreeBuilder(repo)
	if _, err := repoTreeBuilder.WriteTreeFromEntryIDs(map[string]Hash{"file": blobID}); err != nil {
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
