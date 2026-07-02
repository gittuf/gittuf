// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithFetchDepth(t *testing.T) {
	options := &FetchOptions{}
	WithFetchDepth(1)(options)

	assert.Equal(t, 1, options.Depth)
}

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

	t.Run("push to non-existent remote", func(t *testing.T) {
		localTmpDir := t.TempDir()
		localRepo := CreateTestGitRepository(t, localTmpDir, false)

		err := localRepo.PushRefSpec("nonexistent", []string{refSpecs})
		assert.ErrorContains(t, err, "unable to push")
	})
}

func TestPushRepositorySHA256(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"

	localTmpDir := t.TempDir()
	remoteTmpDir := t.TempDir()

	localRepo := CreateTestGitRepository(t, localTmpDir, false, WithSHA256Format())
	remoteRepo := CreateTestGitRepository(t, remoteTmpDir, true, WithSHA256Format())

	if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
		t.Fatal(err)
	}

	emptyBlobHash, err := localRepo.WriteBlob(nil)
	require.Nil(t, err)
	tree, err := NewTreeBuilder(localRepo).WriteTreeFromEntries([]TreeEntry{NewEntryBlob("foo", emptyBlobHash)})
	if err != nil {
		t.Fatal(err)
	}

	if _, err := localRepo.Commit(tree, refName, "Test commit\n", false); err != nil {
		t.Fatal(err)
	}

	err = localRepo.Push(remoteName, []string{refName})
	assert.Nil(t, err)

	localRef, err := localRepo.GetReference(refName)
	require.Nil(t, err)
	remoteRef, err := remoteRepo.GetReference(refName)
	require.Nil(t, err)

	assert.Equal(t, localRef, remoteRef)
	assert.Len(t, localRef.String(), 64)
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

	t.Run("push to non-existent remote", func(t *testing.T) {
		localTmpDir := t.TempDir()
		localRepo := CreateTestGitRepository(t, localTmpDir, false)

		err := localRepo.Push("nonexistent", []string{refName})
		assert.ErrorContains(t, err, "unable to push")
	})

	t.Run("push with invalid ref", func(t *testing.T) {
		localTmpDir := t.TempDir()
		localRepo := CreateTestGitRepository(t, localTmpDir, false)

		err := localRepo.Push("origin", []string{"nonexistent-ref"})
		assert.ErrorIs(t, err, ErrReferenceNotFound)
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

	t.Run("fetch from non-existent remote", func(t *testing.T) {
		localTmpDir := t.TempDir()
		localRepo := CreateTestGitRepository(t, localTmpDir, true)

		err := localRepo.FetchRefSpec("nonexistent", []string{refSpecs})
		assert.ErrorContains(t, err, "unable to fetch")
	})

	t.Run("fetch with depth", func(t *testing.T) {
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir, true)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false)

		require.Nil(t, localRepo.CreateRemote(remoteName, remoteTmpDir))

		tree, err := NewTreeBuilder(remoteRepo).WriteTreeFromEntries(nil)
		require.Nil(t, err)
		remoteRef, err := remoteRepo.Commit(tree, refName, "Test commit\n", false)
		require.Nil(t, err)

		err = localRepo.FetchRefSpec(remoteName, []string{refSpecs}, WithFetchDepth(1))
		require.Nil(t, err)

		localRef, err := localRepo.GetReference(refName)
		require.Nil(t, err)
		assert.Equal(t, remoteRef, localRef)
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

	t.Run("fetch from non-existent remote", func(t *testing.T) {
		localTmpDir := t.TempDir()
		localRepo := CreateTestGitRepository(t, localTmpDir, true)

		err := localRepo.Fetch("nonexistent", []string{refName}, true)
		assert.ErrorContains(t, err, "unable to fetch")
	})

	t.Run("fetch with invalid ref", func(t *testing.T) {
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()
		localRepo := CreateTestGitRepository(t, localTmpDir, true)
		_ = CreateTestGitRepository(t, remoteTmpDir, false)

		err := localRepo.CreateRemote(remoteName, remoteTmpDir)
		require.Nil(t, err)

		err = localRepo.Fetch(remoteName, []string{"nonexistent-ref"}, true)
		assert.ErrorIs(t, err, ErrReferenceNotFound)
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

	t.Run("fetch from non-existent remote", func(t *testing.T) {
		err := downstreamRepo.FetchObject("nonexistent", ZeroHash)
		assert.ErrorContains(t, err, "unable to fetch object")
	})
}

func TestCloneAndFetchRepositoryObjectFormat(t *testing.T) {
	for _, objectFormat := range []ObjectFormat{ObjectFormatSHA1, ObjectFormatSHA256} {
		t.Run(string(objectFormat), func(t *testing.T) {
			remoteTmpDir := t.TempDir()
			localTmpDir := t.TempDir()

			remoteRepo := CreateTestGitRepository(t, remoteTmpDir, false, WithObjectFormat(objectFormat))

			emptyBlobHash, err := remoteRepo.WriteBlob(nil)
			require.Nil(t, err)
			tree, err := NewTreeBuilder(remoteRepo).WriteTreeFromEntries([]TreeEntry{NewEntryBlob("foo", emptyBlobHash)})
			require.Nil(t, err)

			if _, err := remoteRepo.Commit(tree, "refs/heads/main", "Commit to main\n", false); err != nil {
				t.Fatal(err)
			}

			localRepo, err := CloneAndFetchRepository(remoteTmpDir, localTmpDir, "refs/heads/main", nil, false)
			require.Nil(t, err)

			assert.Equal(t, objectFormat, localRepo.GetObjectFormat())
			assert.Equal(t, remoteRepo.ZeroHash(), localRepo.ZeroHash())
		})
	}
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

	t.Run("miscellaneous error checking", func(t *testing.T) {
		_, err := CloneAndFetchRepository("", "", "", nil, false)
		assert.ErrorContains(t, err, "target directory must be specified")

		_, err = CloneAndFetchRepository(filepath.Join(t.TempDir(), "missing"), t.TempDir(), "", nil, false)
		assert.ErrorContains(t, err, "unable to clone repository")
	})
}

func TestCreateRemote(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)
	err := repo.CreateRemote("origin", tmpDir)
	assert.Nil(t, err)

	err = repo.CreateRemote("origin", tmpDir)
	assert.ErrorContains(t, err, "unable to add remote")
}
