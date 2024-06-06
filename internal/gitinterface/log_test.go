// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"sort"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestGetCommitsBetweenRange(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	refName := plumbing.ReferenceName("refs/heads/main")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	ref, err := repo.Reference(refName, true)
	if err != nil {
		t.Fatal(err)
	}

	emptyBlobHash, err := WriteBlob(repo, []byte{})
	if err != nil {
		t.Fatal(err)
	}

	treeHashes := createTestTrees(t, repo, emptyBlobHash, 5)
	if err != nil {
		t.Fatal(err)
	}

	commitIDs := []plumbing.Hash{}
	for i := 0; i < 5; i++ {
		commit := CreateCommitObject(testGitConfig, treeHashes[i], []plumbing.Hash{ref.Hash()}, "Test commit", testClock)
		if _, err := ApplyCommit(repo, commit, ref); err != nil {
			t.Fatal(err)
		}

		ref, err = repo.Reference(refName, true)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs = append(commitIDs, ref.Hash())
	}

	allCommits, err := GetCommitsFromCommitIDs(commitIDs, repo)
	if err != nil {
		t.Fatal(err)
	}
	// Git tree structure with their commit trees and their values:
	//
	// Commit1 <- Commit2 <- Commit3 <- Commit4 <- Commit5
	//   |          |           |           |         |
	// Tree1      Tree2       Tree3       Tree4     Tree5
	//   |          |           |           |         |
	// Blob1      Blob1       Blob1       Blob1      Blob1
	//            Blob2       Blob2       Blob2      Blob2
	//                        Blob3       Blob3      Blob3
	//                                    Blob4      Blob4
	//                                               Blob5

	t.Run("Check range between commits 1 and 5", func(t *testing.T) {
		commits, err := GetCommitsBetweenRange(repo, commitIDs[4], commitIDs[0])
		assert.Nil(t, err)
		expectedCommits := []*object.Commit{allCommits[4], allCommits[3], allCommits[2], allCommits[1]}
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].ID().String() < expectedCommits[j].ID().String()
		})
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Pass in wrong order", func(t *testing.T) {
		// Passing in the wrong order and getting a different result is expected behavior. We can show this with a simple example

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

		commits, err := GetCommitsBetweenRange(repo, commitIDs[0], commitIDs[4])
		assert.Nil(t, err)
		assert.Empty(t, commits)
	})

	t.Run("Get all commits", func(t *testing.T) {
		commits, err := GetCommitsBetweenRange(repo, commitIDs[4], plumbing.ZeroHash)
		assert.Nil(t, err)
		expectedCommits := allCommits
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].ID().String() < expectedCommits[j].ID().String()
		})
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Get commits from invalid range", func(t *testing.T) {
		commits, err := GetCommitsBetweenRange(repo, plumbing.ZeroHash, plumbing.ZeroHash)
		assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)
		assert.Nil(t, commits)
	})
	t.Run("Get commits from non-existent commit", func(t *testing.T) {
		nonExistentHash := EmptyBlob()
		commits, err := GetCommitsBetweenRange(repo, nonExistentHash, plumbing.ZeroHash)
		assert.Nil(t, err)
		assert.Equal(t, commits, []*object.Commit{})
	})
}

func TestGetCommitsBetweenRangeRepo(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir)

	refName := "refs/heads/main"
	treeBuilder := NewReplacementTreeBuilder(repo)

	// Write empty tree
	emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
	if err != nil {
		t.Fatal(err)
	}

	allCommits := []Hash{}
	for i := 0; i < 5; i++ {
		commitHash, err := repo.Commit(emptyTreeID, refName, "Test commit\n", false)
		assert.Nil(t, err)
		t.Log(commitHash)
		allCommits = append(allCommits, commitHash)
	}
	// Git tree structure with their commit trees and their values:
	//
	// Commit1 <- Commit2 <- Commit3 <- Commit4 <- Commit5
	//   |          |           |           |         |
	// Tree1      Tree2       Tree3       Tree4     Tree5
	//   |          |           |           |         |
	//
	//
	//
	//
	//

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
		// Passing in the wrong order and getting a different result is expected behavior. We can show this with a simple example

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

		commits, err := repo.GetCommitsBetweenRange(allCommits[0], allCommits[4])
		assert.Nil(t, err)
		assert.Empty(t, commits)
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
		assert.Equal(t, commits, []Hash{})
	})
}

func TestGetCommitsBetweenRangeForMergeCommits(t *testing.T) {
	// Creating a tree with merge commits
	commitIDs := make([]plumbing.Hash, 0, 6)
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	refName := plumbing.ReferenceName("refs/heads/main")

	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	ref, err := repo.Reference(refName, true)
	if err != nil {
		t.Fatal(err)
	}

	emptyBlobHash, err := WriteBlob(repo, []byte{})
	if err != nil {
		t.Fatal(err)
	}

	treeHashes := createTestTrees(t, repo, emptyBlobHash, 6)
	if err != nil {
		t.Fatal(err)
	}

	// creating the first commit

	commit := CreateCommitObject(testGitConfig, treeHashes[0], []plumbing.Hash{ref.Hash()}, fmt.Sprintf("Test commit %v", 1), testClock)
	commitHash, err := WriteCommit(repo, commit)
	if err != nil {
		t.Fatal(err)
	}

	commitIDs = append(commitIDs, commitHash)

	// creating two children from the first commit

	// in the visual, these will be commit 2 and commit 3

	children := createChildrenCommits(t, repo, treeHashes, commitHash, 2)

	commitIDs = append(commitIDs, children...)

	// creating a child for commit 2, which in the visual will be commit 4

	commit = CreateCommitObject(testGitConfig, treeHashes[3], []plumbing.Hash{children[0]}, fmt.Sprintf("Test commit %v", 4), testClock)

	commitHash, err = WriteCommit(repo, commit)
	if err != nil {
		t.Fatal(err)
	}

	commitIDs = append(commitIDs, commitHash)

	// creating a merge commit from the two children, which in the visual will be commit 5

	commit = CreateCommitObject(testGitConfig, treeHashes[4], children, fmt.Sprintf("Test commit %v", 5), testClock)

	commitHash, err = WriteCommit(repo, commit)
	if err != nil {
		t.Fatal(err)
	}

	commitIDs = append(commitIDs, commitHash)

	// creating a child for commit 3, which in the visual will be commit 6

	commit = CreateCommitObject(testGitConfig, treeHashes[5], []plumbing.Hash{children[1]}, fmt.Sprintf("Test commit %v", 6), testClock)

	commitHash, err = WriteCommit(repo, commit)
	if err != nil {
		t.Fatal(err)
	}
	commitIDs = append(commitIDs, commitHash)

	// Git tree with merge commit structure without its commit trees and its values:
	//
	//  commit 4       commit 5         commit 6
	//    │              │  │              │
	//    └─► commit 2 ◄─┘  └─► commit 3 ◄─┘
	//            │              │
	//            └─► commit 1 ◄─┘

	allCommits, err := GetCommitsFromCommitIDs(commitIDs, repo)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("Test commit 1", func(t *testing.T) {
		// commit 1 is the first commit, so it should be the only commit returned
		commits, err := GetCommitsBetweenRange(repo, commitIDs[0], plumbing.ZeroHash)
		assert.Nil(t, err)
		expectedCommits := []*object.Commit{allCommits[0]}
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Test commit 2", func(t *testing.T) {
		// commit 2 is the first child of commit 1, so only it and commit 1 should be returned
		commits, err := GetCommitsBetweenRange(repo, commitIDs[1], plumbing.ZeroHash)
		assert.Nil(t, err)
		expectedCommits := []*object.Commit{allCommits[1], allCommits[0]}
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Test commit 3", func(t *testing.T) {
		// commit 3 is the second child of commit 1, so only it and commit 1 should be returned
		commits, err := GetCommitsBetweenRange(repo, commitIDs[2], plumbing.ZeroHash)
		assert.Nil(t, err)
		expectedCommits := []*object.Commit{allCommits[0], allCommits[2]}
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Test commit 4", func(t *testing.T) {
		// commit 4 is the child of commit 2, so only it, commit 2, and commit 2's parent commit 1 should be returned
		commits, err := GetCommitsBetweenRange(repo, commitIDs[3], plumbing.ZeroHash)
		assert.Nil(t, err)
		expectedCommits := []*object.Commit{allCommits[1], allCommits[0], allCommits[3]}
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Test commit 5, the merge commit", func(t *testing.T) {
		// commit 5 is the merge commit of commit 2 and commit 3, so it should return commit 5, commit 2, commit 3, and commit 1 (the parent of commit 2 and commit 3)
		commits, err := GetCommitsBetweenRange(repo, commitIDs[4], plumbing.ZeroHash)
		assert.Nil(t, err)
		expectedCommits := []*object.Commit{allCommits[4], allCommits[1], allCommits[0], allCommits[2]}
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Test commit 6", func(t *testing.T) {
		// commit 6 is the child of commit 3, so it should return commit 6, commit 3, and commit 1 (the parent of commit 3)
		commits, err := GetCommitsBetweenRange(repo, commitIDs[5], plumbing.ZeroHash)
		assert.Nil(t, err)
		expectedCommits := []*object.Commit{allCommits[0], allCommits[5], allCommits[2]}
		assert.Equal(t, expectedCommits, commits)
	})
}

func GetCommitsFromCommitIDs(commitIDs []plumbing.Hash, repo *git.Repository) ([]*object.Commit, error) {
	allCommits := make([]*object.Commit, 0, len(commitIDs))
	for _, commitID := range commitIDs {
		commit, err := GetCommit(repo, commitID)
		if err != nil {
			return nil, err
		}

		allCommits = append(allCommits, commit)
	}

	return allCommits, nil
}

func createTestTrees(t *testing.T, repo *git.Repository, emptyBlobHash plumbing.Hash, num int) []plumbing.Hash {
	t.Helper()
	treeHashes := make([]plumbing.Hash, 0, num)
	for i := 1; i <= num; i++ {
		objects := make([]object.TreeEntry, 0, i)
		for j := 0; j < i; j++ {
			objects = append(objects, object.TreeEntry{Name: fmt.Sprintf("%d", j+1), Hash: emptyBlobHash})
		}

		treeHash, err := WriteTree(repo, objects)
		if err != nil {
			t.Fatal(err)
		}

		treeHashes = append(treeHashes, treeHash)
	}
	return treeHashes
}

func createChildrenCommits(t *testing.T, repo *git.Repository, treeHashes []plumbing.Hash, parentHash plumbing.Hash, numChildren int) []plumbing.Hash {
	t.Helper()

	children := make([]plumbing.Hash, 0, numChildren)

	for i := 1; i <= numChildren; i++ {
		commit := CreateCommitObject(testGitConfig, treeHashes[i], []plumbing.Hash{parentHash}, fmt.Sprintf("Test commit %v", i+1), testClock)

		commitHash, err := WriteCommit(repo, commit)
		if err != nil {
			t.Fatal(err)
		}
		children = append(children, commitHash)
	}
	return children
}
