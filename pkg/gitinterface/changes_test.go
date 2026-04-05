// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetFilePathsChangedByCommitRepository(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	treeBuilder := NewTreeBuilder(repo)

	blobIDs := []Hash{}
	for i := 0; i < 3; i++ {
		blobID, err := repo.WriteBlob([]byte(fmt.Sprintf("%d", i)))
		if err != nil {
			t.Fatal(err)
		}
		blobIDs = append(blobIDs, blobID)
	}

	emptyTree, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	// In each of the tests below, repo.Commit uses the test name as a ref
	// This allows us to use a single repo in all the tests without interference
	// For example, if we use a single repo and a single ref (say main), the test that
	// expects a commit with no parents will have a parent because of a commit created
	// in a previous test

	t.Run("modify single file", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[1])})
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		diffs, err := repo.GetFilePathsChangedByCommit(cB)
		assert.Nil(t, err)
		assert.Equal(t, []string{"a"}, diffs)
	})

	t.Run("rename single file", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("b", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		diffs, err := repo.GetFilePathsChangedByCommit(cB)
		assert.Nil(t, err)
		assert.Equal(t, []string{"a", "b"}, diffs)
	})

	t.Run("swap two files around", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0]), NewEntryBlob("b", blobIDs[1])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[1]), NewEntryBlob("b", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		diffs, err := repo.GetFilePathsChangedByCommit(cB)
		assert.Nil(t, err)
		assert.Equal(t, []string{"a", "b"}, diffs)
	})

	t.Run("create new file", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0]), NewEntryBlob("b", blobIDs[1])})
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		diffs, err := repo.GetFilePathsChangedByCommit(cB)
		assert.Nil(t, err)
		assert.Equal(t, []string{"b"}, diffs)
	})

	t.Run("delete file", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0]), NewEntryBlob("b", blobIDs[1])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		diffs, err := repo.GetFilePathsChangedByCommit(cB)
		assert.Nil(t, err)
		assert.Equal(t, []string{"b"}, diffs)
	})

	t.Run("modify file and create new file", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[2]), NewEntryBlob("b", blobIDs[1])})
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		diffs, err := repo.GetFilePathsChangedByCommit(cB)
		assert.Nil(t, err)
		assert.Equal(t, []string{"a", "b"}, diffs)
	})

	t.Run("no parent", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		cA, err := repo.Commit(treeA, testNameToRefName(t.Name()), "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		diffs, err := repo.GetFilePathsChangedByCommit(cA)
		assert.Nil(t, err)
		assert.Equal(t, []string{"a"}, diffs)
	})

	t.Run("merge commit with commit matching parent", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[1])})
		if err != nil {
			t.Fatal(err)
		}

		mainBranch := testNameToRefName(t.Name())
		featureBranch := testNameToRefName(t.Name() + " feature branch")

		// Write common commit for both branches
		cCommon, err := repo.Commit(emptyTree, mainBranch, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.SetReference(featureBranch, cCommon); err != nil {
			t.Fatal(err)
		}

		cA, err := repo.Commit(treeA, mainBranch, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, featureBranch, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		// Create a merge commit with two parents
		cM := repo.commitWithParents(t, treeB, []Hash{cA, cB}, "Merge commit\n", false)

		diffs, err := repo.GetFilePathsChangedByCommit(cM)
		assert.Nil(t, err)
		assert.Nil(t, diffs)
	})

	t.Run("merge commit with no matching parent", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("b", blobIDs[1])})
		if err != nil {
			t.Fatal(err)
		}

		treeC, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("c", blobIDs[2])})
		if err != nil {
			t.Fatal(err)
		}

		mainBranch := testNameToRefName(t.Name())
		featureBranch := testNameToRefName(t.Name() + " feature branch")

		// Write common commit for both branches
		cCommon, err := repo.Commit(emptyTree, mainBranch, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.SetReference(featureBranch, cCommon); err != nil {
			t.Fatal(err)
		}

		cA, err := repo.Commit(treeA, mainBranch, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, featureBranch, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		// Create a merge commit with two parents and a different tree
		cM := repo.commitWithParents(t, treeC, []Hash{cA, cB}, "Merge commit\n", false)

		diffs, err := repo.GetFilePathsChangedByCommit(cM)
		assert.Nil(t, err)
		assert.Equal(t, []string{"a", "b", "c"}, diffs)
	})

	t.Run("merge commit with overlapping parent trees", func(t *testing.T) {
		treeA, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[0])})
		if err != nil {
			t.Fatal(err)
		}

		treeB, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[1])})
		if err != nil {
			t.Fatal(err)
		}

		treeC, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{NewEntryBlob("a", blobIDs[2])})
		if err != nil {
			t.Fatal(err)
		}

		mainBranch := testNameToRefName(t.Name())
		featureBranch := testNameToRefName(t.Name() + " feature branch")

		// Write common commit for both branches
		cCommon, err := repo.Commit(emptyTree, mainBranch, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.SetReference(featureBranch, cCommon); err != nil {
			t.Fatal(err)
		}

		cA, err := repo.Commit(treeA, mainBranch, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		cB, err := repo.Commit(treeB, featureBranch, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		// Create a merge commit with two parents and an overlapping tree
		cM := repo.commitWithParents(t, treeC, []Hash{cA, cB}, "Merge commit\n", false)

		diffs, err := repo.GetFilePathsChangedByCommit(cM)
		assert.Nil(t, err)
		assert.Equal(t, []string{"a"}, diffs)
	})

	t.Run("error with blob instead of commit", func(t *testing.T) {
		blobID, err := repo.WriteBlob([]byte("test"))
		if err != nil {
			t.Fatal(err)
		}

		_, err = repo.GetFilePathsChangedByCommit(blobID)
		assert.NotNil(t, err)
		assert.Contains(t, err.Error(), "is not a commit object")
	})

	t.Run("error with non-existent commit", func(t *testing.T) {
		_, err := repo.GetFilePathsChangedByCommit(ZeroHash)
		assert.NotNil(t, err)
	})

	t.Run("commit with no changes", func(t *testing.T) {
		emptyTree2, err := treeBuilder.WriteTreeFromEntries(nil)
		require.Nil(t, err)

		commit1, err := repo.Commit(emptyTree2, "refs/heads/no-change", "First commit\n", false)
		require.Nil(t, err)

		commit2, err := repo.Commit(emptyTree2, "refs/heads/no-change", "Second commit\n", false)
		require.Nil(t, err)

		diffs, err := repo.GetFilePathsChangedByCommit(commit2)
		assert.Nil(t, err)
		assert.Nil(t, diffs)

		_, err = repo.GetFilePathsChangedByCommit(commit1)
		assert.Nil(t, err)
	})

	t.Run("commit with multiple file changes", func(t *testing.T) {
		blob1, err := repo.WriteBlob([]byte("content1"))
		require.Nil(t, err)
		blob2, err := repo.WriteBlob([]byte("content2"))
		require.Nil(t, err)
		blob3, err := repo.WriteBlob([]byte("content3"))
		require.Nil(t, err)

		tree1, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("file1.txt", blob1),
		})
		require.Nil(t, err)

		tree2, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("file1.txt", blob2),
			NewEntryBlob("file2.txt", blob2),
			NewEntryBlob("file3.txt", blob3),
		})
		require.Nil(t, err)

		_, err = repo.Commit(tree1, "refs/heads/multi-change", "First commit\n", false)
		require.Nil(t, err)

		commit2, err := repo.Commit(tree2, "refs/heads/multi-change", "Second commit\n", false)
		require.Nil(t, err)

		diffs, err := repo.GetFilePathsChangedByCommit(commit2)
		assert.Nil(t, err)
		assert.Len(t, diffs, 3)
	})

	t.Run("nested directory structure", func(t *testing.T) {
		blob1, err := repo.WriteBlob([]byte("content1"))
		require.Nil(t, err)

		blob2, err := repo.WriteBlob([]byte("content2"))
		require.Nil(t, err)

		tree, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("dir1/file1.txt", blob1),
			NewEntryBlob("dir1/dir2/file2.txt", blob2),
		})
		require.Nil(t, err)

		commitID, err := repo.Commit(tree, "refs/heads/nested", "Nested files\n", false)
		require.Nil(t, err)

		paths, err := repo.GetFilePathsChangedByCommit(commitID)
		assert.Nil(t, err)
		assert.Contains(t, paths, "dir1/file1.txt")
		assert.Contains(t, paths, "dir1/dir2/file2.txt")
	})

	t.Run("merge commit with changes", func(t *testing.T) {
		blob1, err := repo.WriteBlob([]byte("content1"))
		require.Nil(t, err)
		blob2, err := repo.WriteBlob([]byte("content2"))
		require.Nil(t, err)
		blob3, err := repo.WriteBlob([]byte("content3"))
		require.Nil(t, err)

		tree1, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("file1.txt", blob1),
		})
		require.Nil(t, err)

		commit1, err := repo.Commit(tree1, "refs/heads/merge-main", "Initial commit\n", false)
		require.Nil(t, err)

		tree2, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("file1.txt", blob1),
			NewEntryBlob("file2.txt", blob2),
		})
		require.Nil(t, err)

		commit2, err := repo.Commit(tree2, "refs/heads/merge-branch1", "Add file2\n", false)
		require.Nil(t, err)

		if err := repo.SetReference("refs/heads/merge-branch2", commit1); err != nil {
			t.Fatal(err)
		}

		tree3, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("file1.txt", blob1),
			NewEntryBlob("file3.txt", blob3),
		})
		require.Nil(t, err)

		commit3, err := repo.Commit(tree3, "refs/heads/merge-branch2", "Add file3\n", false)
		require.Nil(t, err)

		treeMerge, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("file1.txt", blob1),
			NewEntryBlob("file2.txt", blob2),
			NewEntryBlob("file3.txt", blob3),
		})
		require.Nil(t, err)

		mergeCommit := repo.commitWithParents(t, treeMerge, []Hash{commit2, commit3}, "Merge branches\n", false)

		paths, err := repo.GetFilePathsChangedByCommit(mergeCommit)
		assert.Nil(t, err)
		assert.NotEmpty(t, paths)
	})

	t.Run("merge commit with no changes from last parent", func(t *testing.T) {
		blob1, err := repo.WriteBlob([]byte("mc1"))
		require.Nil(t, err)

		tree1, err := treeBuilder.WriteTreeFromEntries([]TreeEntry{
			NewEntryBlob("f1.txt", blob1),
		})
		require.Nil(t, err)

		c1, err := repo.Commit(tree1, "refs/heads/mc-main", "Initial\n", false)
		require.Nil(t, err)

		c2, err := repo.Commit(tree1, "refs/heads/mc-branch", "Same\n", false)
		require.Nil(t, err)

		mergeCommitNoChange := repo.commitWithParents(t, tree1, []Hash{c1, c2}, "Merge with no change\n", false)

		paths, err := repo.GetFilePathsChangedByCommit(mergeCommitNoChange)
		assert.Nil(t, err)
		assert.Nil(t, paths)
	})
}
