// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPushRefSpecRepository(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refSpecs := fmt.Sprintf("%s:%s", refName, refName)

	t.Run("assert remote repo does not have object until it is pushed", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, false)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, true)

		localTreeBuilder := NewTreeBuilder(localRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the local repository
		emptyBlobHash, err := localRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := localTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		// Check that the tree is not present on the remote repository
		_, err = remoteRepo.GetAllFilesInTree(tree)
		assert.Contains(t, err.Error(), "fatal: not a tree object") // tree doesn't exist

		if _, err := localRepo.Commit(tree, refName, "Test commit\n", false); err != nil {
			t.Fatal(err)
		}

		err = localRepo.PushRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)

		expectedFiles := map[string]Hash{"foo": emptyBlobHash}
		remoteEntries, err := remoteRepo.GetAllFilesInTree(tree)
		assert.Nil(t, err)
		assert.Equal(t, expectedFiles, remoteEntries)
	})

	t.Run("assert after push that src and dst refs match", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, false)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, true)

		localTreeBuilder := NewTreeBuilder(localRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the local repository
		emptyBlobHash, err := localRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := localTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := localRepo.Commit(tree, refName, "Test commit\n", false); err != nil {
			t.Fatal(err)
		}

		err = localRepo.PushRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)

		localRef, err := localRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		remoteRef, err := remoteRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, localRef, remoteRef)
	})

	t.Run("assert no error when there are no updates to push", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, false)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, true)

		localTreeBuilder := NewTreeBuilder(localRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the local repository
		emptyBlobHash, err := localRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := localTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := localRepo.Commit(tree, refName, "Test commit\n", false); err != nil {
			t.Fatal(err)
		}

		err = localRepo.PushRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)

		localRef, err := localRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		remoteRef, err := remoteRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, localRef, remoteRef)

		// Push again; nothing to push
		err = localRepo.PushRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)
	})
}

func TestPushRepository(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"

	t.Run("assert remote repo does not have object until it is pushed", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, false)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, true)

		localTreeBuilder := NewTreeBuilder(localRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the local repository
		emptyBlobHash, err := localRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := localTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		// Check that the tree is not present on the remote repository
		_, err = remoteRepo.GetAllFilesInTree(tree)
		assert.Contains(t, err.Error(), "fatal: not a tree object") // tree doesn't exist

		if _, err := localRepo.Commit(tree, refName, "Test commit\n", false); err != nil {
			t.Fatal(err)
		}

		err = localRepo.Push(remoteName, []string{refName})
		assert.Nil(t, err)

		expectedFiles := map[string]Hash{"foo": emptyBlobHash}
		remoteEntries, err := remoteRepo.GetAllFilesInTree(tree)
		assert.Nil(t, err)
		assert.Equal(t, expectedFiles, remoteEntries)
	})

	t.Run("assert after push that src and dst refs match", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, false)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, true)

		localTreeBuilder := NewTreeBuilder(localRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the local repository
		emptyBlobHash, err := localRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := localTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := localRepo.Commit(tree, refName, "Test commit\n", false); err != nil {
			t.Fatal(err)
		}

		err = localRepo.Push(remoteName, []string{refName})
		assert.Nil(t, err)

		localRef, err := localRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		remoteRef, err := remoteRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, localRef, remoteRef)
	})

	t.Run("assert no error when there are no updates to push", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, false)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, true)

		localTreeBuilder := NewTreeBuilder(localRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the local repository
		emptyBlobHash, err := localRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := localTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		if _, err := localRepo.Commit(tree, refName, "Test commit\n", false); err != nil {
			t.Fatal(err)
		}

		err = localRepo.Push(remoteName, []string{refName})
		assert.Nil(t, err)

		localRef, err := localRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		remoteRef, err := remoteRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, localRef, remoteRef)

		// Push again; nothing to push
		err = localRepo.Push(remoteName, []string{refName})
		assert.Nil(t, err)
	})
}

func TestFetchRefSpecRepository(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refSpecs := fmt.Sprintf("+%s:%s", refName, refName)

	t.Run("assert local repo does not have object until fetched", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, true)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the remote repository
		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		// Check that the tree is not present on the local repository
		_, err = localRepo.GetAllFilesInTree(tree)
		assert.Contains(t, err.Error(), "fatal: not a tree object") // tree doesn't exist

		_, err = remoteRepo.Commit(tree, refName, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.FetchRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)

		expectedFiles := map[string]Hash{"foo": emptyBlobHash}
		localEntries, err := localRepo.GetAllFilesInTree(tree)
		assert.Nil(t, err)
		assert.Equal(t, expectedFiles, localEntries)
	})

	t.Run("assert after fetch that both refs match", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, true)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the remote repository
		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		_, err = remoteRepo.Commit(tree, refName, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.FetchRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)

		localRef, err := localRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		remoteRef, err := remoteRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, localRef, remoteRef)
	})

	t.Run("assert no error when there are no updates to fetch", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, true)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the remote repository
		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		_, err = remoteRepo.Commit(tree, refName, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.FetchRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)

		localRef, err := localRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		remoteRef, err := remoteRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, localRef, remoteRef)

		// Fetch again, nothing to fetch
		err = localRepo.FetchRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)

		newLocalRef, err := localRepo.GetReference(refName)
		require.Nil(t, err)
		assert.Equal(t, localRef, newLocalRef)
	})
}

func TestFetchRepository(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"

	t.Run("assert local repo does not have object until fetched", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, true)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the remote repository
		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		// Check that the tree is not present on the local repository
		_, err = localRepo.GetAllFilesInTree(tree)
		assert.Contains(t, err.Error(), "fatal: not a tree object") // tree doesn't exist

		_, err = remoteRepo.Commit(tree, refName, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.Fetch(remoteName, []string{refName}, true)
		assert.Nil(t, err)

		expectedFiles := map[string]Hash{"foo": emptyBlobHash}
		localEntries, err := localRepo.GetAllFilesInTree(tree)
		assert.Nil(t, err)
		assert.Equal(t, expectedFiles, localEntries)
	})

	t.Run("assert after fetch that both refs match", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, true)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the remote repository
		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		_, err = remoteRepo.Commit(tree, refName, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.Fetch(remoteName, []string{refName}, true)
		assert.Nil(t, err)

		localRef, err := localRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		remoteRef, err := remoteRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, localRef, remoteRef)
	})

	t.Run("assert no error when there are no updates to fetch", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, true)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create a tree in the remote repository
		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		_, err = remoteRepo.Commit(tree, refName, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.Fetch(remoteName, []string{refName}, true)
		assert.Nil(t, err)

		localRef, err := localRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		remoteRef, err := remoteRepo.GetReference(refName)
		if err != nil {
			t.Fatal(err)
		}

		assert.Equal(t, localRef, remoteRef)

		// Fetch again, nothing to fetch
		err = localRepo.Fetch(remoteName, []string{refName}, true)
		assert.Nil(t, err)

		newLocalRef, err := localRepo.GetReference(refName)
		require.Nil(t, err)
		assert.Equal(t, localRef, newLocalRef)
	})
}

func TestFetchObject(t *testing.T) {
	tmpDir1 := t.TempDir()
	upstreamRepo := CreateTestGitRepository(t, tmpDir1, true)
	err := upstreamRepo.SetGitConfig("uploadpack.allowReachableSHA1InWant", "true")
	require.Nil(t, err)
	treeBuilder := NewTreeBuilder(upstreamRepo)
	treeID, err := treeBuilder.WriteTreeFromEntries(nil)
	require.Nil(t, err)
	commitID, err := upstreamRepo.Commit(treeID, "refs/heads/main", "Initial commit\n", false)
	require.Nil(t, err)

	tmpDir2 := t.TempDir()
	downstreamRepo := CreateTestGitRepository(t, tmpDir2, false)
	err = downstreamRepo.AddRemote("origin", tmpDir1)
	require.Nil(t, err)

	has := downstreamRepo.HasObject(commitID)
	assert.False(t, has)

	err = downstreamRepo.FetchObject("origin", commitID)
	assert.Nil(t, err)

	has = downstreamRepo.HasObject(commitID)
	assert.True(t, has)
}

func TestCloneAndFetchRepository(t *testing.T) {
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	t.Run("clone and fetch remote repository, verify refs match, not bare", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		mainCommit, err := remoteRepo.Commit(tree, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}
		otherCommit, err := remoteRepo.Commit(tree, anotherRefName, "Commit to feature", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.SetReference("HEAD", mainCommit); err != nil {
			t.Fatal(err)
		}

		localRepo, err := CloneAndFetchRepository(remoteTmpDir, localTmpDir, refName, []string{anotherRefName}, false)
		if err != nil {
			t.Fatal(err)
		}

		localMainCommit, err := localRepo.GetReference(refName)
		assert.Nil(t, err)
		localOtherCommit, err := localRepo.GetReference(anotherRefName)
		assert.Nil(t, err)

		assert.Equal(t, mainCommit, localMainCommit)
		assert.Equal(t, otherCommit, localOtherCommit)

		assert.True(t, strings.HasSuffix(localRepo.gitDirPath, ".git"))
		dirEntries, err := os.ReadDir(strings.TrimSuffix(localRepo.gitDirPath, ".git"))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "foo", dirEntries[1].Name()) // [0] will be the entry for the .git directory
	})

	t.Run("clone and fetch remote repository without specifying initial branch, verify refs match, not bare", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		mainCommit, err := remoteRepo.Commit(tree, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}
		otherCommit, err := remoteRepo.Commit(tree, anotherRefName, "Commit to feature", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.SetReference("HEAD", mainCommit); err != nil {
			t.Fatal(err)
		}

		localRepo, err := CloneAndFetchRepository(remoteTmpDir, localTmpDir, "", []string{anotherRefName}, false)
		if err != nil {
			t.Fatal(err)
		}

		localMainCommit, err := localRepo.GetReference(refName)
		assert.Nil(t, err)
		localOtherCommit, err := localRepo.GetReference(anotherRefName)
		assert.Nil(t, err)

		assert.Equal(t, mainCommit, localMainCommit)
		assert.Equal(t, otherCommit, localOtherCommit)

		assert.True(t, strings.HasSuffix(localRepo.gitDirPath, ".git"))
		dirEntries, err := os.ReadDir(strings.TrimSuffix(localRepo.gitDirPath, ".git"))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "foo", dirEntries[1].Name()) // [0] will be the entry for the .git directory
	})

	t.Run("clone and fetch remote repository with only one ref, verify refs match, not bare", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		mainCommit, err := remoteRepo.Commit(tree, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.SetReference("HEAD", mainCommit); err != nil {
			t.Fatal(err)
		}

		localRepo, err := CloneAndFetchRepository(remoteTmpDir, localTmpDir, "", []string{}, false)
		if err != nil {
			t.Fatal(err)
		}

		localMainCommit, err := localRepo.GetReference(refName)
		assert.Nil(t, err)
		assert.Equal(t, mainCommit, localMainCommit)

		assert.True(t, strings.HasSuffix(localRepo.gitDirPath, ".git"))
		dirEntries, err := os.ReadDir(strings.TrimSuffix(localRepo.gitDirPath, ".git"))
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "foo", dirEntries[1].Name()) // [0] will be the entry for the .git directory
	})

	t.Run("clone and fetch remote repository, verify refs match, bare", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		mainCommit, err := remoteRepo.Commit(tree, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}
		otherCommit, err := remoteRepo.Commit(tree, anotherRefName, "Commit to feature", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.SetReference("HEAD", mainCommit); err != nil {
			t.Fatal(err)
		}

		localRepo, err := CloneAndFetchRepository(remoteTmpDir, localTmpDir, refName, []string{anotherRefName}, true)
		if err != nil {
			t.Fatal(err)
		}

		localMainCommit, err := localRepo.GetReference(refName)
		assert.Nil(t, err)
		localOtherCommit, err := localRepo.GetReference(anotherRefName)
		assert.Nil(t, err)

		assert.Equal(t, mainCommit, localMainCommit)
		assert.Equal(t, otherCommit, localOtherCommit)

		assert.False(t, strings.HasSuffix(localRepo.gitDirPath, ".git"))
		dirEntries, err := os.ReadDir(localRepo.gitDirPath)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "FETCH_HEAD", dirEntries[0].Name())
	})

	t.Run("clone and fetch remote repository without specifying initial branch, verify refs match, bare", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		mainCommit, err := remoteRepo.Commit(tree, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}
		otherCommit, err := remoteRepo.Commit(tree, anotherRefName, "Commit to feature", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.SetReference("HEAD", mainCommit); err != nil {
			t.Fatal(err)
		}

		localRepo, err := CloneAndFetchRepository(remoteTmpDir, localTmpDir, "", []string{anotherRefName}, true)
		if err != nil {
			t.Fatal(err)
		}

		localMainCommit, err := localRepo.GetReference(refName)
		assert.Nil(t, err)
		localOtherCommit, err := localRepo.GetReference(anotherRefName)
		assert.Nil(t, err)

		assert.Equal(t, mainCommit, localMainCommit)
		assert.Equal(t, otherCommit, localOtherCommit)

		assert.False(t, strings.HasSuffix(localRepo.gitDirPath, ".git"))
		dirEntries, err := os.ReadDir(localRepo.gitDirPath)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "FETCH_HEAD", dirEntries[0].Name())
	})

	t.Run("clone and fetch remote repository with only one ref, verify refs match, bare", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		remoteTreeBuilder := NewTreeBuilder(remoteRepo)

		emptyBlobHash, err := remoteRepo.WriteBlob(nil)
		require.Nil(t, err)
		entries := []TreeEntry{NewEntryBlob("foo", emptyBlobHash)}

		tree, err := remoteTreeBuilder.WriteTreeFromEntries(entries)
		if err != nil {
			t.Fatal(err)
		}

		mainCommit, err := remoteRepo.Commit(tree, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.SetReference("HEAD", mainCommit); err != nil {
			t.Fatal(err)
		}

		localRepo, err := CloneAndFetchRepository(remoteTmpDir, localTmpDir, "", []string{}, true)
		if err != nil {
			t.Fatal(err)
		}

		localMainCommit, err := localRepo.GetReference(refName)
		assert.Nil(t, err)
		assert.Equal(t, mainCommit, localMainCommit)

		assert.False(t, strings.HasSuffix(localRepo.gitDirPath, ".git"))
		dirEntries, err := os.ReadDir(localRepo.gitDirPath)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, "FETCH_HEAD", dirEntries[0].Name())
	})
}
