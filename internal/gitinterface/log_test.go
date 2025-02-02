// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGetCommitsBetweenRangeRepository(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	refName := "refs/heads/main"
	treeBuilder := NewTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	allCommits := []Hash{}
	for i := 0; i < 5; i++ {
		commitHash, err := repo.Commit(emptyTreeID, refName, "Test commit\n", false)
		require.Nil(t, err)
		allCommits = append(allCommits, commitHash)
	}

	// Git tree structure with their commit trees and their values:
	//
	// Commit1 <- Commit2 <- Commit3 <- Commit4 <- Commit5

	t.Run("Check range between commits 1 and 5", func(t *testing.T) {
		commits, err := repo.GetCommitsBetweenRange(allCommits[4], allCommits[0])
		assert.Nil(t, err)

		expectedCommits := []Hash{allCommits[4], allCommits[3], allCommits[2], allCommits[1]}
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})

		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Pass in wrong order", func(t *testing.T) {
		commits, err := repo.GetCommitsBetweenRange(allCommits[0], allCommits[4])
		assert.Nil(t, err)
		assert.Empty(t, commits)
	})

	t.Run("Check range in separate branches", func(t *testing.T) {
		//     7
		//    ↙ ↘
		//   5   6
		//   ↓   ↓
		//   3   4
		//   ↓   ↓
		//   1   2
		//    ↘ ↙
		//     0

		// If we pass in 7 and 1, we expect to get 7, 6, 5, 4, 3, and 2
		// If we pass in 1 and 7, we should expect nothing since every node that
		// is in the subtree of 1 is also in the subtree of 7

		// Create two new branches for this
		mainBranch := testNameToRefName(t.Name())
		featureBranch := testNameToRefName(t.Name() + " feature branch")

		// Add a common commit for both
		commonCommit, err := repo.Commit(emptyTreeID, mainBranch, "Initial commit\n", false)
		require.Nil(t, err)
		if err := repo.SetReference(featureBranch, commonCommit); err != nil {
			t.Fatal(err)
		}

		mainBranchCommits := []Hash{}
		for i := 0; i < 5; i++ {
			commitHash, err := repo.Commit(emptyTreeID, mainBranch, fmt.Sprintf("Main commit %d\n", i), false)
			require.Nil(t, err)
			mainBranchCommits = append(mainBranchCommits, commitHash)
		}

		featureBranchCommits := []Hash{}
		for i := 0; i < 5; i++ {
			commitHash, err := repo.Commit(emptyTreeID, featureBranch, fmt.Sprintf("Feature commit %d\n", i), false)
			require.Nil(t, err)
			featureBranchCommits = append(featureBranchCommits, commitHash)
		}

		// Add a common merge commit
		mergeCommit := repo.commitWithParents(
			t,
			emptyTreeID,
			[]Hash{
				mainBranchCommits[len(mainBranchCommits)-1],
				featureBranchCommits[len(featureBranchCommits)-1],
			},
			"Merge branches\n",
			false,
		)

		// Check merge to first commit in main branch (not initial common commit)
		expectedCommits := append([]Hash{mergeCommit}, mainBranchCommits[1:]...)
		expectedCommits = append(expectedCommits, featureBranchCommits...)
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})
		commits, err := repo.GetCommitsBetweenRange(mergeCommit, mainBranchCommits[0])
		assert.Nil(t, err)
		assert.Equal(t, expectedCommits, commits)

		// Check merge to first commit in feature branch (not initial common commit)
		expectedCommits = append([]Hash{mergeCommit}, featureBranchCommits[1:]...)
		expectedCommits = append(expectedCommits, mainBranchCommits...)
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})
		commits, err = repo.GetCommitsBetweenRange(mergeCommit, featureBranchCommits[0])
		assert.Nil(t, err)
		assert.Equal(t, expectedCommits, commits)

		// Check merge to initial common commit
		expectedCommits = append([]Hash{mergeCommit}, mainBranchCommits...)
		expectedCommits = append(expectedCommits, featureBranchCommits...)
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})
		commits, err = repo.GetCommitsBetweenRange(mergeCommit, commonCommit)
		assert.Nil(t, err)
		assert.Equal(t, expectedCommits, commits)

		// Set both branches to merge commit, diverge again
		if err := repo.SetReference(mainBranch, mergeCommit); err != nil {
			t.Fatal(err)
		}
		if err := repo.SetReference(featureBranch, mergeCommit); err != nil {
			t.Fatal(err)
		}

		mainBranchCommits = []Hash{}
		for i := 0; i < 5; i++ {
			commitHash, err := repo.Commit(emptyTreeID, mainBranch, fmt.Sprintf("Main commit %d\n", i), false)
			require.Nil(t, err)
			mainBranchCommits = append(mainBranchCommits, commitHash)
		}

		featureBranchCommits = []Hash{}
		for i := 0; i < 5; i++ {
			commitHash, err := repo.Commit(emptyTreeID, featureBranch, fmt.Sprintf("Feature commit %d\n", i), false)
			require.Nil(t, err)
			featureBranchCommits = append(featureBranchCommits, commitHash)
		}

		newMergeCommit := repo.commitWithParents(
			t,
			emptyTreeID,
			[]Hash{
				mainBranchCommits[len(mainBranchCommits)-1],
				featureBranchCommits[len(featureBranchCommits)-1],
			},
			"Merge branches\n",
			false,
		)

		// Check range between two merge commits
		expectedCommits = append([]Hash{newMergeCommit}, mainBranchCommits...)
		expectedCommits = append(expectedCommits, featureBranchCommits...)
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})
		commits, err = repo.GetCommitsBetweenRange(newMergeCommit, mergeCommit)
		assert.Nil(t, err)
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Get all commits", func(t *testing.T) {
		commits, err := repo.GetCommitsBetweenRange(allCommits[4], ZeroHash)
		assert.Nil(t, err)

		expectedCommits := allCommits
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Get commits from invalid range", func(t *testing.T) {
		_, err := repo.GetCommitsBetweenRange(ZeroHash, ZeroHash)
		assert.NotNil(t, err)
	})

	t.Run("Get commits from non-existent commit", func(t *testing.T) {
		nonExistentHash, err := repo.WriteBlob([]byte{})
		assert.Nil(t, err)

		commits, err := repo.GetCommitsBetweenRange(nonExistentHash, ZeroHash)
		assert.Nil(t, err)
		assert.Empty(t, commits)
	})
}

func TestGetCommitsBetweenRangeForMergeCommits(t *testing.T) {
	// Creating a tree with merge commits
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	commitIDs := make([]Hash, 0, 6)

	emptyBlobHash, err := repo.WriteBlob(nil)
	if err != nil {
		t.Fatal(err)
	}

	treeHashes := createTestTrees(t, repo, emptyBlobHash, 6)
	if err != nil {
		t.Fatal(err)
	}

	// creating the first commit
	commitID := repo.commitWithParents(t, treeHashes[0], nil, fmt.Sprintf("Test commit %v", 1), false)
	commitIDs = append(commitIDs, commitID)

	// creating two children from the first commit
	// in the visual, these will be commit 2 and commit 3
	children := createChildrenCommits(t, repo, treeHashes, commitID, 2)
	commitIDs = append(commitIDs, children...)

	// creating a child for commit 2, which in the visual will be commit 4
	commitID = repo.commitWithParents(t, treeHashes[3], []Hash{children[0]}, fmt.Sprintf("Test commit %v", 4), false)
	commitIDs = append(commitIDs, commitID)

	// creating a merge commit from the two children, which in the visual will be commit 5
	commitID = repo.commitWithParents(t, treeHashes[4], children, fmt.Sprintf("Test commit %v", 5), false)
	commitIDs = append(commitIDs, commitID)

	// creating a child for commit 3, which in the visual will be commit 6
	commitID = repo.commitWithParents(t, treeHashes[5], []Hash{children[1]}, fmt.Sprintf("Test commit %v", 6), false)
	commitIDs = append(commitIDs, commitID)

	// Git tree with merge commit structure without its commit trees and its values:
	//
	//  commit 4       commit 5         commit 6
	//    │              │  │              │
	//    └─► commit 2 ◄─┘  └─► commit 3 ◄─┘
	//            │              │
	//            └─► commit 1 ◄─┘

	t.Run("Test commit 1", func(t *testing.T) {
		// commit 1 is the first commit, so it should be the only commit returned
		commits, err := repo.GetCommitsBetweenRange(commitIDs[0], ZeroHash)
		assert.Nil(t, err)
		expectedCommits := []Hash{commitIDs[0]}
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Test commit 2", func(t *testing.T) {
		// commit 2 is the first child of commit 1, so only it and commit 1 should be returned
		commits, err := repo.GetCommitsBetweenRange(commitIDs[1], ZeroHash)
		assert.Nil(t, err)

		expectedCommits := []Hash{commitIDs[1], commitIDs[0]}
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})

		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Test commit 3", func(t *testing.T) {
		// commit 3 is the second child of commit 1, so only it and commit 1 should be returned
		commits, err := repo.GetCommitsBetweenRange(commitIDs[2], ZeroHash)
		assert.Nil(t, err)

		expectedCommits := []Hash{commitIDs[0], commitIDs[2]}
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})

		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Test commit 4", func(t *testing.T) {
		// commit 4 is the child of commit 2, so only it, commit 2, and commit 2's parent commit 1 should be returned
		commits, err := repo.GetCommitsBetweenRange(commitIDs[3], ZeroHash)
		assert.Nil(t, err)

		expectedCommits := []Hash{commitIDs[1], commitIDs[0], commitIDs[3]}
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})

		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Test commit 5, the merge commit", func(t *testing.T) {
		// commit 5 is the merge commit of commit 2 and commit 3, so it should return commit 5, commit 2, commit 3, and commit 1 (the parent of commit 2 and commit 3)
		commits, err := repo.GetCommitsBetweenRange(commitIDs[4], ZeroHash)
		assert.Nil(t, err)

		expectedCommits := []Hash{commitIDs[4], commitIDs[1], commitIDs[0], commitIDs[2]}
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})

		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Test commit 6", func(t *testing.T) {
		// commit 6 is the child of commit 3, so it should return commit 6, commit 3, and commit 1 (the parent of commit 3)
		commits, err := repo.GetCommitsBetweenRange(commitIDs[5], ZeroHash)
		assert.Nil(t, err)

		expectedCommits := []Hash{commitIDs[0], commitIDs[5], commitIDs[2]}
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].String() < expectedCommits[j].String()
		})

		assert.Equal(t, expectedCommits, commits)
	})
}

func createTestTrees(t *testing.T, repo *Repository, emptyBlobHash Hash, num int) []Hash {
	t.Helper()
	treeBuilder := NewTreeBuilder(repo)
	treeHashes := make([]Hash, 0, num)
	for i := 1; i <= num; i++ {
		objects := []TreeEntry{}
		for j := 0; j < i; j++ {
			objects = append(objects, NewEntryBlob(fmt.Sprintf("%d", j+1), emptyBlobHash))
		}

		treeHash, err := treeBuilder.WriteTreeFromEntries(objects)
		if err != nil {
			t.Fatal(err)
		}

		treeHashes = append(treeHashes, treeHash)
	}
	return treeHashes
}

func createChildrenCommits(t *testing.T, repo *Repository, treeHashes []Hash, parentHash Hash, numChildren int) []Hash {
	t.Helper()

	children := make([]Hash, 0, numChildren)

	for i := 1; i <= numChildren; i++ {
		commitID := repo.commitWithParents(t, treeHashes[i], []Hash{parentHash}, fmt.Sprintf("Test commit %v", i+1), false)
		children = append(children, commitID)
	}
	return children
}
