// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepositoryEmptyTree(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	hash, err := repo.EmptyTree()
	assert.Nil(t, err)

	// SHA-1 ID used by Git to denote an empty tree
	// $ git hash-object -t tree --stdin < /dev/null
	assert.Equal(t, "4b825dc642cb6eb9a060e54bf8d69288fbee4904", hash.String())
}

func TestGetMergeTree(t *testing.T) {
	t.Run("no conflict", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		// We meed to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		emptyBlobID, err := repo.WriteBlob(nil)
		if err != nil {
			t.Fatal(err)
		}

		treeBuilder := NewTreeBuilder(repo)
		emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		treeAID, err := treeBuilder.WriteRootTreeFromBlobIDs(map[string]Hash{"a": emptyBlobID})
		if err != nil {
			t.Fatal(err)
		}
		treeBID, err := treeBuilder.WriteRootTreeFromBlobIDs(map[string]Hash{"b": emptyBlobID})
		if err != nil {
			t.Fatal(err)
		}
		combinedTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(map[string]Hash{
			"a": emptyBlobID,
			"b": emptyBlobID,
		})
		if err != nil {
			t.Fatal(err)
		}

		mainRef := "refs/heads/main"
		featureRef := "refs/heads/feature"

		// Add commits to the main branch
		baseCommitID, err := repo.Commit(emptyTreeID, mainRef, "Initial commit", false)
		if err != nil {
			t.Fatal(err)
		}
		commitAID, err := repo.Commit(treeAID, mainRef, "Commit A", false)
		if err != nil {
			t.Fatal(err)
		}

		// Add commits to the feature branch
		if err := repo.SetReference(featureRef, baseCommitID); err != nil {
			t.Fatal(err)
		}
		commitBID, err := repo.Commit(treeBID, featureRef, "Commit B", false)
		if err != nil {
			t.Fatal(err)
		}

		// fix up checked out worktree
		if _, err := repo.executor("restore", "--staged", ".").executeString(); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.executor("checkout", "--", ".").executeString(); err != nil {
			t.Fatal(err)
		}

		mergeTreeID, err := repo.GetMergeTree(commitAID, commitBID)
		assert.Nil(t, err)
		if !combinedTreeID.Equal(mergeTreeID) {
			mergeTreeContents, err := repo.GetAllFilesInTree(mergeTreeID)
			if err != nil {
				t.Fatalf("unexpected error when debugging non-matched merge trees: %s", err.Error())
			}
			t.Log("merge tree contents:", mergeTreeContents)
			t.Error("merge trees don't match")
		}
	})

	t.Run("merge conflict", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		// We meed to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		emptyBlobID, err := repo.WriteBlob(nil)
		if err != nil {
			t.Fatal(err)
		}

		treeBuilder := NewTreeBuilder(repo)
		emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		if err != nil {
			t.Fatal(err)
		}

		blobAID, err := repo.WriteBlob([]byte("a"))
		if err != nil {
			t.Fatal(err)
		}
		blobBID, err := repo.WriteBlob([]byte("b"))
		if err != nil {
			t.Fatal(err)
		}

		treeAID, err := treeBuilder.WriteRootTreeFromBlobIDs(map[string]Hash{"a": blobAID})
		if err != nil {
			t.Fatal(err)
		}
		treeBID, err := treeBuilder.WriteRootTreeFromBlobIDs(map[string]Hash{
			"a": blobBID,
			"b": emptyBlobID,
		})
		if err != nil {
			t.Fatal(err)
		}

		mainRef := "refs/heads/main"
		featureRef := "refs/heads/feature"

		// Add commits to the main branch
		baseCommitID, err := repo.Commit(emptyTreeID, mainRef, "Initial commit", false)
		if err != nil {
			t.Fatal(err)
		}
		commitAID, err := repo.Commit(treeAID, mainRef, "Commit A", false)
		if err != nil {
			t.Fatal(err)
		}

		// Add commits to the feature branch
		if err := repo.SetReference(featureRef, baseCommitID); err != nil {
			t.Fatal(err)
		}
		commitBID, err := repo.Commit(treeBID, featureRef, "Commit B", false)
		if err != nil {
			t.Fatal(err)
		}

		// fix up checked out worktree
		if _, err := repo.executor("restore", "--staged", ".").executeString(); err != nil {
			t.Fatal(err)
		}
		if _, err := repo.executor("checkout", "--", ".").executeString(); err != nil {
			t.Fatal(err)
		}

		_, err = repo.GetMergeTree(commitAID, commitBID)
		assert.NotNil(t, err)
	})
}

func TestTreeBuilder(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	blobAID, err := repo.WriteBlob([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}

	blobBID, err := repo.WriteBlob([]byte("b"))
	if err != nil {
		t.Fatal(err)
	}

	emptyTreeID := "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

	t.Run("no blobs", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)
		treeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		assert.Nil(t, err)
		assert.Equal(t, emptyTreeID, treeID.String())

		treeID, err = treeBuilder.WriteRootTreeFromBlobIDs(map[string]Hash{})
		assert.Nil(t, err)
		assert.Equal(t, emptyTreeID, treeID.String())
	})

	t.Run("both blobs in the root directory", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := map[string]Hash{
			"a": blobAID,
			"b": blobBID,
		}

		rootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(input)
		assert.Nil(t, err)

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, input, files)
	})

	t.Run("both blobs in same subdirectory", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := map[string]Hash{
			"dir/a": blobAID,
			"dir/b": blobBID,
		}

		rootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(input)
		assert.Nil(t, err)

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, input, files)
	})

	t.Run("both blobs in different subdirectories", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := map[string]Hash{
			"foo/a": blobAID,
			"bar/b": blobBID,
		}

		rootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(input)
		assert.Nil(t, err)

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, input, files)
	})

	t.Run("blobs in mix of root directory and subdirectories", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := map[string]Hash{
			"a":                blobAID,
			"foo/bar/foobar/b": blobBID,
		}

		rootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(input)
		assert.Nil(t, err)

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, input, files)
	})
}
