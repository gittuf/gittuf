// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
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

func TestGetPathIDInTree(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)
	treeBuilder := NewTreeBuilder(repo)

	blobAID, err := repo.WriteBlob([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}

	blobBID, err := repo.WriteBlob([]byte("b"))
	if err != nil {
		t.Fatal(err)
	}

	emptyTreeID := "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

	t.Run("no items", func(t *testing.T) {
		treeID, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, emptyTreeID, treeID.String())

		pathID, err := repo.GetPathIDInTree("a", treeID)
		assert.ErrorIs(t, err, ErrTreeDoesNotHavePath)
		assert.Nil(t, pathID)
	})

	t.Run("no subdirectories", func(t *testing.T) {
		exhaustiveItems := []TreeEntry{
			NewEntryBlob("a", blobAID),
			NewEntryBlob("b", blobBID),
		}

		treeID, err := treeBuilder.WriteTreeFromEntries(exhaustiveItems)
		if err != nil {
			t.Fatal(err)
		}

		itemID, err := repo.GetPathIDInTree("a", treeID)
		assert.Nil(t, err)
		assert.Equal(t, blobAID, itemID)
	})

	t.Run("one file in root tree, one file in subdirectory", func(t *testing.T) {
		exhaustiveItems := []TreeEntry{
			NewEntryBlob("foo/a", blobAID),
			NewEntryBlob("b", blobBID),
		}

		treeID, err := treeBuilder.WriteTreeFromEntries(exhaustiveItems)
		if err != nil {
			t.Fatal(err)
		}

		itemID, err := repo.GetPathIDInTree("foo/a", treeID)
		assert.Nil(t, err)
		assert.Equal(t, blobAID, itemID)
	})

	t.Run("multiple levels", func(t *testing.T) {
		exhaustiveItems := []TreeEntry{
			NewEntryBlob("foo/bar/foobar/a", blobAID),
			NewEntryBlob("foobar/foo/bar/b", blobBID),
		}

		treeID, err := treeBuilder.WriteTreeFromEntries(exhaustiveItems)
		if err != nil {
			t.Fatal(err)
		}

		// find tree ID for foo/bar/foobar
		expectedItemID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobAID)})
		if err != nil {
			t.Fatal(err)
		}

		itemID, err := repo.GetPathIDInTree("foo/bar/foobar", treeID)
		assert.Nil(t, err)
		assert.Equal(t, expectedItemID, itemID)

		// find tree ID for foo/bar
		expectedItemID, err = treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("foobar/a", blobAID)})
		if err != nil {
			t.Fatal(err)
		}

		itemID, err = repo.GetPathIDInTree("foo/bar", treeID)
		assert.Nil(t, err)
		assert.Equal(t, expectedItemID, itemID)

		// find tree ID for foobar/foo
		expectedItemID, err = treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("bar/b", blobBID)})
		if err != nil {
			t.Fatal(err)
		}

		itemID, err = repo.GetPathIDInTree("foobar/foo", treeID)
		assert.Nil(t, err)
		assert.Equal(t, expectedItemID, itemID)

		itemID, err = repo.GetPathIDInTree("foobar/foo/foobar", treeID)
		assert.ErrorIs(t, err, ErrTreeDoesNotHavePath)
		assert.Nil(t, itemID)
	})
}

func TestGetTreeItems(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)
	treeBuilder := NewTreeBuilder(repo)

	blobAID, err := repo.WriteBlob([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}

	blobBID, err := repo.WriteBlob([]byte("b"))
	if err != nil {
		t.Fatal(err)
	}

	emptyTreeID := "4b825dc642cb6eb9a060e54bf8d69288fbee4904"

	t.Run("no items", func(t *testing.T) {
		treeID, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, emptyTreeID, treeID.String())

		treeItems, err := repo.GetTreeItems(treeID)
		assert.Nil(t, err)
		assert.Nil(t, treeItems)
	})

	t.Run("no subdirectories", func(t *testing.T) {
		exhaustiveItems := []TreeEntry{
			NewEntryBlob("a", blobAID),
			NewEntryBlob("b", blobBID),
		}

		treeID, err := treeBuilder.WriteTreeFromEntries(exhaustiveItems)
		if err != nil {
			t.Fatal(err)
		}

		expectedOutput := map[string]Hash{
			"a": blobAID,
			"b": blobBID,
		}

		treeItems, err := repo.GetTreeItems(treeID)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, treeItems)
	})

	t.Run("one file in root tree, one file in subdirectory", func(t *testing.T) {
		exhaustiveItems := []TreeEntry{
			NewEntryBlob("foo/a", blobAID),
			NewEntryBlob("b", blobBID),
		}

		treeID, err := treeBuilder.WriteTreeFromEntries(exhaustiveItems)
		if err != nil {
			t.Fatal(err)
		}

		fooTreeID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobAID)})
		if err != nil {
			t.Fatal(err)
		}

		expectedTreeItems := map[string]Hash{
			"foo": fooTreeID,
			"b":   blobBID,
		}

		treeItems, err := repo.GetTreeItems(treeID)
		assert.Nil(t, err)
		assert.Equal(t, expectedTreeItems, treeItems)
	})

	t.Run("one file in foo tree, one file in bar", func(t *testing.T) {
		exhaustiveItems := []TreeEntry{
			NewEntryBlob("foo/a", blobAID),
			NewEntryBlob("bar/b", blobBID),
		}

		treeID, err := treeBuilder.WriteTreeFromEntries(exhaustiveItems)
		if err != nil {
			t.Fatal(err)
		}

		fooTreeID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobAID)})
		if err != nil {
			t.Fatal(err)
		}

		barTreeID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("b", blobBID)})
		if err != nil {
			t.Fatal(err)
		}

		expectedTreeItems := map[string]Hash{
			"foo": fooTreeID,
			"bar": barTreeID,
		}

		treeItems, err := repo.GetTreeItems(treeID)
		assert.Nil(t, err)
		assert.Equal(t, expectedTreeItems, treeItems)
	})
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
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		treeAID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", emptyBlobID)})
		if err != nil {
			t.Fatal(err)
		}
		treeBID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("b", emptyBlobID)})
		if err != nil {
			t.Fatal(err)
		}
		combinedTreeID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("a", emptyBlobID),
			NewEntryBlob("b", emptyBlobID),
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
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
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

		treeAID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobAID)})
		if err != nil {
			t.Fatal(err)
		}
		treeBID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("a", blobBID),
			NewEntryBlob("b", emptyBlobID),
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

	t.Run("fast forward merge", func(t *testing.T) {
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
		treeID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("empty", emptyBlobID)})
		if err != nil {
			t.Fatal(err)
		}

		commitID, err := repo.Commit(treeID, "refs/heads/main", "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		mergeTreeID, err := repo.GetMergeTree(ZeroHash, commitID)
		assert.Nil(t, err)
		assert.Equal(t, treeID, mergeTreeID)
	})
}

func TestCreateSubtreeFromUpstreamRepository(t *testing.T) {
	t.Run("subtree into HEAD", func(t *testing.T) {
		tmpDir1 := t.TempDir()
		downstreamRepository := CreateTestGitRepository(t, tmpDir1, false)

		blobAID, err := downstreamRepository.WriteBlob([]byte("a"))
		require.Nil(t, err)

		blobBID, err := downstreamRepository.WriteBlob([]byte("b"))
		require.Nil(t, err)

		downstreamTreeBuilder := NewTreeBuilder(downstreamRepository)

		// The downstream tree (if set as exists in test below) is:
		// oof/a -> blobA
		// b     -> blobB
		downstreamTreeEntries := []TreeEntry{
			NewEntryBlob("oof/a", blobAID),
			NewEntryBlob("b", blobBID),
		}
		downstreamTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries(downstreamTreeEntries)
		require.Nil(t, err)

		downstreamCommitID, err := downstreamRepository.Commit(downstreamTreeID, "refs/heads/main", "Initial commit\n", false)
		require.Nil(t, err)

		err = downstreamRepository.SetSymbolicReference("HEAD", "refs/heads/main")
		require.Nil(t, err)

		downstreamRepository.RestoreWorktree(t)

		tmpDir2 := t.TempDir()
		upstreamRepository := CreateTestGitRepository(t, tmpDir2, true)

		_, err = upstreamRepository.WriteBlob([]byte("a"))
		require.Nil(t, err)

		_, err = upstreamRepository.WriteBlob([]byte("b"))
		require.Nil(t, err)

		upstreamTreeBuilder := NewTreeBuilder(upstreamRepository)

		// The upstream tree is:
		// a                -> blobA
		// foo/a            -> blobA
		// foo/b            -> blobB
		// foobar/foo/bar/b -> blobB

		upstreamTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("a", blobAID),
			NewEntryBlob("foo/a", blobAID),
			NewEntryBlob("foo/b", blobBID),
			NewEntryBlob("foobar/foo/bar/b", blobBID),
		})
		require.Nil(t, err)

		upstreamRef := "refs/heads/main"
		upstreamCommitID, err := upstreamRepository.Commit(upstreamTreeID, upstreamRef, "Initial commit\n", false)
		require.Nil(t, err)

		downstreamCommitIDNew, err := downstreamRepository.CreateSubtreeFromUpstreamRepository(upstreamRepository, upstreamCommitID, "", "refs/heads/main", "upstream")
		assert.Nil(t, err)
		assert.NotEqual(t, downstreamCommitID, downstreamCommitIDNew)

		statuses, err := downstreamRepository.Status()
		require.Nil(t, err)
		assert.Empty(t, statuses)
	})

	t.Run("various other subtree scenarios", func(t *testing.T) {
		tmpDir1 := t.TempDir()
		downstreamRepository := CreateTestGitRepository(t, tmpDir1, false)

		blobAID, err := downstreamRepository.WriteBlob([]byte("a"))
		require.Nil(t, err)

		blobBID, err := downstreamRepository.WriteBlob([]byte("b"))
		require.Nil(t, err)

		downstreamTreeBuilder := NewTreeBuilder(downstreamRepository)

		// The downstream tree (if set as exists in test below) is:
		// oof/a -> blobA
		// b     -> blobB
		downstreamTreeEntries := []TreeEntry{
			NewEntryBlob("oof/a", blobAID),
			NewEntryBlob("b", blobBID),
		}
		downstreamTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries(downstreamTreeEntries)
		require.Nil(t, err)

		tmpDir2 := t.TempDir()
		upstreamRepository := CreateTestGitRepository(t, tmpDir2, true)

		_, err = upstreamRepository.WriteBlob([]byte("a"))
		require.Nil(t, err)

		_, err = upstreamRepository.WriteBlob([]byte("b"))
		require.Nil(t, err)

		upstreamTreeBuilder := NewTreeBuilder(upstreamRepository)

		// The upstream tree is:
		// a                -> blobA
		// foo/a            -> blobA
		// foo/b            -> blobB
		// foobar/foo/bar/b -> blobB

		upstreamRootTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("a", blobAID),
			NewEntryBlob("foo/a", blobAID),
			NewEntryBlob("foo/b", blobBID),
			NewEntryBlob("foobar/foo/bar/b", blobBID),
		})
		require.Nil(t, err)

		upstreamRef := "refs/heads/main"
		upstreamCommitID, err := upstreamRepository.Commit(upstreamRootTreeID, upstreamRef, "Initial commit\n", false)
		require.Nil(t, err)

		tests := map[string]struct {
			upstreamPath     string
			localPath        string
			refExists        bool // refExists -> we must check for other files but no prior propagation has happened
			priorPropagation bool // priorPropagation -> localPath is already populated, mutually exclusive with refExists
			err              error
		}{
			"directory in root, ref does not exist": {
				localPath:        "upstream",
				refExists:        false,
				priorPropagation: false,
			},
			"directory in root, trailing slash, ref does not exist": {
				localPath:        "upstream/",
				refExists:        false,
				priorPropagation: false,
			},
			"directory in root, ref exists": {
				localPath:        "upstream",
				refExists:        true,
				priorPropagation: false,
			},
			"directory in root, trailing slash, ref exists": {
				localPath:        "upstream/",
				refExists:        true,
				priorPropagation: false,
			},
			"directory in root, prior propagation exists": {
				localPath:        "upstream",
				refExists:        false,
				priorPropagation: true,
			},
			"directory in root, trailing slash, prior propagation exists": {
				localPath:        "upstream/",
				refExists:        false,
				priorPropagation: true,
			},
			"directory in subdirectory, ref does not exist": {
				localPath:        "foo/upstream",
				refExists:        false,
				priorPropagation: false,
			},
			"directory in subdirectory, trailing slash, ref does not exist": {
				localPath:        "foo/upstream/",
				refExists:        false,
				priorPropagation: false,
			},
			"directory in subdirectory, ref exists": {
				localPath:        "foo/upstream",
				refExists:        true,
				priorPropagation: false,
			},
			"directory in subdirectory, trailing slash, ref exists": {
				localPath:        "foo/upstream/",
				refExists:        true,
				priorPropagation: false,
			},
			"directory in subdirectory, prior propagation exists": {
				localPath:        "foo/upstream",
				refExists:        false,
				priorPropagation: true,
			},
			"directory in subdirectory, trailing slash, prior propagation exists": {
				localPath:        "foo/upstream/",
				refExists:        false,
				priorPropagation: true,
			},
			"with upstream path, directory in root, ref does not exist": {
				upstreamPath:     "foo",
				localPath:        "upstream",
				refExists:        false,
				priorPropagation: false,
			},
			"with upstream path, directory in root, trailing slash, ref does not exist": {
				upstreamPath:     "foo/",
				localPath:        "upstream/",
				refExists:        false,
				priorPropagation: false,
			},
			"with upstream path, directory in root, ref exists": {
				upstreamPath:     "foo",
				localPath:        "upstream",
				refExists:        true,
				priorPropagation: false,
			},
			"with upstream path, directory in root, trailing slash, ref exists": {
				upstreamPath:     "foo/",
				localPath:        "upstream/",
				refExists:        true,
				priorPropagation: false,
			},
			"with upstream path, directory in root, prior propagation exists": {
				upstreamPath:     "foo",
				localPath:        "upstream",
				refExists:        false,
				priorPropagation: true,
			},
			"with upstream path, directory in root, trailing slash, prior propagation exists": {
				upstreamPath:     "foo/",
				localPath:        "upstream/",
				refExists:        false,
				priorPropagation: true,
			},
			"with upstream path, directory in subdirectory, ref does not exist": {
				upstreamPath:     "foo",
				localPath:        "foo/upstream",
				refExists:        false,
				priorPropagation: false,
			},
			"with upstream path, directory in subdirectory, trailing slash, ref does not exist": {
				upstreamPath:     "foo/",
				localPath:        "foo/upstream/",
				refExists:        false,
				priorPropagation: false,
			},
			"with upstream path, directory in subdirectory, ref exists": {
				upstreamPath:     "foo",
				localPath:        "foo/upstream",
				refExists:        true,
				priorPropagation: false,
			},
			"with upstream path, directory in subdirectory, trailing slash, ref exists": {
				upstreamPath:     "foo/",
				localPath:        "foo/upstream/",
				refExists:        true,
				priorPropagation: false,
			},
			"with upstream path, directory in subdirectory, prior propagation exists": {
				upstreamPath:     "foo",
				localPath:        "foo/upstream",
				refExists:        false,
				priorPropagation: true,
			},
			"with upstream path, directory in subdirectory, trailing slash, prior propagation exists": {
				upstreamPath:     "foo",
				localPath:        "foo/upstream/",
				refExists:        false,
				priorPropagation: true,
			},
			"with upstream path as subdirectory, directory in root, ref does not exist": {
				upstreamPath:     "foobar/foo",
				localPath:        "upstream",
				refExists:        false,
				priorPropagation: false,
			},
			"with upstream path as subdirectory, directory in root, trailing slash, ref does not exist": {
				upstreamPath:     "foobar/foo/",
				localPath:        "upstream/",
				refExists:        false,
				priorPropagation: false,
			},
			"with upstream path as subdirectory, directory in root, ref exists": {
				upstreamPath:     "foobar/foo",
				localPath:        "upstream",
				refExists:        true,
				priorPropagation: false,
			},
			"with upstream path as subdirectory, directory in root, trailing slash, ref exists": {
				upstreamPath:     "foobar/foo/",
				localPath:        "upstream/",
				refExists:        true,
				priorPropagation: false,
			},
			"with upstream path as subdirectory, directory in root, prior propagation exists": {
				upstreamPath:     "foobar/foo",
				localPath:        "upstream",
				refExists:        false,
				priorPropagation: true,
			},
			"with upstream path as subdirectory, directory in root, trailing slash, prior propagation exists": {
				upstreamPath:     "foobar/foo/",
				localPath:        "upstream/",
				refExists:        false,
				priorPropagation: true,
			},
			"with upstream path as subdirectory, directory in subdirectory, ref does not exist": {
				upstreamPath:     "foobar/foo",
				localPath:        "foo/upstream",
				refExists:        false,
				priorPropagation: false,
			},
			"with upstream path as subdirectory, directory in subdirectory, trailing slash, ref does not exist": {
				upstreamPath:     "foobar/foo/",
				localPath:        "foo/upstream/",
				refExists:        false,
				priorPropagation: false,
			},
			"with upstream path as subdirectory, directory in subdirectory, ref exists": {
				upstreamPath:     "foobar/foo",
				localPath:        "foo/upstream",
				refExists:        true,
				priorPropagation: false,
			},
			"with upstream path as subdirectory, directory in subdirectory, trailing slash, ref exists": {
				upstreamPath:     "foobar/foo/",
				localPath:        "foo/upstream/",
				refExists:        true,
				priorPropagation: false,
			},
			"with upstream path as subdirectory, directory in subdirectory, prior propagation exists": {
				upstreamPath:     "foobar/foo",
				localPath:        "foo/upstream",
				refExists:        false,
				priorPropagation: true,
			},
			"with upstream path as subdirectory, directory in subdirectory, trailing slash, prior propagation exists": {
				upstreamPath:     "foobar/foo/",
				localPath:        "foo/upstream/",
				refExists:        false,
				priorPropagation: true,
			},
			"upstream path does not exist": {
				upstreamPath: "does-not-exist",
				localPath:    "foo/upstream/",
				err:          ErrTreeDoesNotHavePath,
			},
			"empty localPath": {
				err: ErrCannotCreateSubtreeIntoRootTree,
			},
		}

		for name, test := range tests {
			t.Run(name, func(t *testing.T) {
				require.False(t, test.refExists && test.priorPropagation, "refExists and priorPropagation can't both be true")

				downstreamRef := testNameToRefName(name)

				if test.refExists {
					_, err := downstreamRepository.Commit(downstreamTreeID, downstreamRef, "Initial commit\n", false)
					require.Nil(t, err)
				} else if test.priorPropagation {
					// We set the upstream path to contain the same tree as the
					// downstreamTree, so:
					// oof/a            -> blobA
					// b                -> blobB
					// <upstream>/oof/a -> blobA
					// <upstream>/b     -> blobB

					entries := []TreeEntry{NewEntryTree(test.localPath, downstreamTreeID)}
					entries = append(entries, downstreamTreeEntries...)

					rootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries(entries)
					require.Nil(t, err)

					_, err = downstreamRepository.Commit(rootTreeID, downstreamRef, "Initial commit\n", false)
					require.Nil(t, err)
				}

				downstreamCommitID, err := downstreamRepository.CreateSubtreeFromUpstreamRepository(upstreamRepository, upstreamCommitID, test.upstreamPath, downstreamRef, test.localPath)
				if test.err != nil {
					assert.ErrorIs(t, err, test.err)
				} else {
					assert.Nil(t, err)

					rootTreeID, err := downstreamRepository.GetCommitTreeID(downstreamCommitID)
					require.Nil(t, err)

					itemID, err := downstreamRepository.GetPathIDInTree(test.localPath, rootTreeID)
					require.Nil(t, err)

					upstreamTreeID := upstreamRootTreeID
					if test.upstreamPath != "" {
						upstreamTreeID, err = upstreamRepository.GetPathIDInTree(test.upstreamPath, upstreamRootTreeID)
						require.Nil(t, err)
					}
					assert.Equal(t, upstreamTreeID, itemID)

					if test.refExists {
						// check that other items are still present
						itemID, err := downstreamRepository.GetPathIDInTree("oof/a", downstreamTreeID)
						require.Nil(t, err)
						assert.Equal(t, blobAID, itemID)

						itemID, err = downstreamRepository.GetPathIDInTree("b", downstreamTreeID)
						require.Nil(t, err)
						assert.Equal(t, blobBID, itemID)
					}

					// We don't need to similarly check when test.priorPropagation is
					// true because we already checked that those contents don't exist
					// in that localPath when we checked the tree ID patches
					// upstreamTreeID
				}
			})
		}
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
		treeID, err := treeBuilder.WriteTreeFromEntries(nil)
		assert.Nil(t, err)
		assert.Equal(t, emptyTreeID, treeID.String())

		treeID, err = treeBuilder.WriteTreeFromEntries(nil)
		assert.Nil(t, err)
		assert.Equal(t, emptyTreeID, treeID.String())
	})

	t.Run("both blobs in the root directory", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := []TreeEntry{
			NewEntryBlob("a", blobAID),
			NewEntryBlob("b", blobBID),
		}

		rootTreeID, err := treeBuilder.WriteTreeFromEntries(input)
		assert.Nil(t, err)

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		expectedOutput := map[string]Hash{
			"a": blobAID,
			"b": blobBID,
		}
		assert.Equal(t, expectedOutput, files)
	})

	t.Run("both blobs in same subdirectory", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := []TreeEntry{
			NewEntryBlob("dir/a", blobAID),
			NewEntryBlob("dir/b", blobBID),
		}

		rootTreeID, err := treeBuilder.WriteTreeFromEntries(input)
		assert.Nil(t, err)

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		expectedOutput := map[string]Hash{
			"dir/a": blobAID,
			"dir/b": blobBID,
		}

		assert.Equal(t, expectedOutput, files)
	})

	t.Run("same blobs in the multiple directories", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := []TreeEntry{
			NewEntryBlob("a", blobAID),
			NewEntryBlob("b", blobBID),
			NewEntryBlob("foo/a", blobAID),
			NewEntryBlob("foo/b", blobBID),
			NewEntryBlob("bar/a", blobAID),
			NewEntryBlob("bar/b", blobBID),
		}

		rootTreeID, err := treeBuilder.WriteTreeFromEntries(input)
		assert.Nil(t, err)

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		expectedOutput := map[string]Hash{
			"a":     blobAID,
			"b":     blobBID,
			"foo/a": blobAID,
			"foo/b": blobBID,
			"bar/a": blobAID,
			"bar/b": blobBID,
		}

		assert.Equal(t, expectedOutput, files)
	})

	t.Run("both blobs in different subdirectories", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := []TreeEntry{
			NewEntryBlob("foo/a", blobAID),
			NewEntryBlob("bar/b", blobBID),
		}

		rootTreeID, err := treeBuilder.WriteTreeFromEntries(input)
		assert.Nil(t, err)

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		expectedOutput := map[string]Hash{
			"foo/a": blobAID,
			"bar/b": blobBID,
		}

		assert.Equal(t, expectedOutput, files)
	})

	t.Run("blobs in mix of root directory and subdirectories", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := []TreeEntry{
			NewEntryBlob("a", blobAID),
			NewEntryBlob("foo/bar/foobar/b", blobBID),
		}

		rootTreeID, err := treeBuilder.WriteTreeFromEntries(input)
		assert.Nil(t, err)

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		expectedOutput := map[string]Hash{
			"a":                blobAID,
			"foo/bar/foobar/b": blobBID,
		}

		assert.Equal(t, expectedOutput, files)
	})

	t.Run("build tree from intermediate tree", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		intermediateTreeInput := []TreeEntry{
			NewEntryBlob("a", blobAID),
		}

		intermediateTreeID, err := treeBuilder.WriteTreeFromEntries(intermediateTreeInput)
		assert.Nil(t, err)

		rootTreeInput := []TreeEntry{
			NewEntryTree("intermediate", intermediateTreeID),
			NewEntryBlob("b", blobBID),
		}

		rootTreeID, err := treeBuilder.WriteTreeFromEntries(rootTreeInput)
		assert.Nil(t, err)

		expectedFiles := map[string]Hash{
			"intermediate/a": blobAID,
			"b":              blobBID,
		}

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, expectedFiles, files)
	})

	t.Run("build tree from nested intermediate tree", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		intermediateTreeInput := []TreeEntry{
			NewEntryBlob("a", blobAID),
		}

		intermediateTreeID, err := treeBuilder.WriteTreeFromEntries(intermediateTreeInput)
		assert.Nil(t, err)

		rootTreeInput := []TreeEntry{
			NewEntryTree("foo/intermediate", intermediateTreeID),
			NewEntryBlob("b", blobBID),
		}

		rootTreeID, err := treeBuilder.WriteTreeFromEntries(rootTreeInput)
		assert.Nil(t, err)

		expectedFiles := map[string]Hash{
			"foo/intermediate/a": blobAID,
			"b":                  blobBID,
		}

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, expectedFiles, files)
	})

	t.Run("build tree from nested multi-level intermediate tree", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		intermediateTreeInput := []TreeEntry{
			NewEntryBlob("intermediate/a", blobAID),
		}

		intermediateTreeID, err := treeBuilder.WriteTreeFromEntries(intermediateTreeInput)
		assert.Nil(t, err)

		rootTreeInput := []TreeEntry{
			NewEntryTree("foo/intermediate", intermediateTreeID),
			NewEntryBlob("b", blobBID),
		}

		rootTreeID, err := treeBuilder.WriteTreeFromEntries(rootTreeInput)
		assert.Nil(t, err)

		expectedFiles := map[string]Hash{
			"foo/intermediate/intermediate/a": blobAID,
			"b":                               blobBID,
		}

		files, err := repo.GetAllFilesInTree(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, expectedFiles, files)
	})
}

func TestEnsureIsTree(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, true)

	blobID, err := repo.WriteBlob([]byte("foo"))
	if err != nil {
		t.Fatal(err)
	}

	treeBuilder := NewTreeBuilder(repo)
	treeID, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("foo", blobID)})
	if err != nil {
		t.Fatal(err)
	}

	err = repo.ensureIsTree(treeID)
	assert.Nil(t, err)

	err = repo.ensureIsTree(blobID)
	assert.NotNil(t, err)
}
