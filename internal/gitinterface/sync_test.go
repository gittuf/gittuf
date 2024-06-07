// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"fmt"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestPushRefSpec(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refSpecs := []config.RefSpec{config.RefSpec(fmt.Sprintf("%s:%s", refName, refName))}
	refNameTyped := plumbing.ReferenceName(refName)

	t.Run("assert remote repo does not have object until it is pushed", func(t *testing.T) {
		// The source repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote repo so we have a URL for it
		tmpDir := t.TempDir()

		repoRemote, err := git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Check that the empty tree object we'll later push to the remote repo
		// is not present
		_, err = repoRemote.Object(plumbing.TreeObject, EmptyTree())
		assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)

		emptyTreeHash, err := WriteTree(repoLocal, []object.TreeEntry{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Commit(repoLocal, emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}

		err = PushRefSpec(context.Background(), repoLocal, remoteName, refSpecs)
		assert.Nil(t, err)

		// This time, the empty tree object must also be in the remote repo
		_, err = repoRemote.Object(plumbing.TreeObject, EmptyTree())
		assert.Nil(t, err)
	})

	t.Run("assert after push that src and dst refs match", func(t *testing.T) {
		// The local repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote repo so we have a URL for it
		tmpDir := t.TempDir()

		repoRemote, err := git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		emptyTreeHash, err := WriteTree(repoLocal, []object.TreeEntry{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Commit(repoLocal, emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}

		err = PushRefSpec(context.Background(), repoLocal, remoteName, refSpecs)
		assert.Nil(t, err)

		refLocal, err := repoLocal.Reference(refNameTyped, true)
		if err != nil {
			t.Fatal(err)
		}
		refRemote, err := repoRemote.Reference(refNameTyped, true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, refLocal.Hash(), refRemote.Hash())
	})

	t.Run("assert no error when there are no updates to push", func(t *testing.T) {
		// The local repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote so we have a URL for it
		tmpDir := t.TempDir()

		_, err = git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		err = PushRefSpec(context.Background(), repoLocal, remoteName, refSpecs)
		assert.Nil(t, err) // no error when it's already up to date
	})
}

func TestPush(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refNameTyped := plumbing.ReferenceName(refName)

	t.Run("assert remote repo does not have object until it is pushed", func(t *testing.T) {
		// The source repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote repo so we have a URL for it
		tmpDir := t.TempDir()

		repoRemote, err := git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Check that the empty tree object we'll later push to the remote repo
		// is not present
		_, err = repoRemote.Object(plumbing.TreeObject, EmptyTree())
		assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)

		emptyTreeHash, err := WriteTree(repoLocal, []object.TreeEntry{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Commit(repoLocal, emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}

		err = Push(context.Background(), repoLocal, remoteName, []string{refName})
		assert.Nil(t, err)

		// This time, the empty tree object must also be in the remote repo
		_, err = repoRemote.Object(plumbing.TreeObject, EmptyTree())
		assert.Nil(t, err)
	})

	t.Run("assert after push that src and dst refs match", func(t *testing.T) {
		// The local repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote repo so we have a URL for it
		tmpDir := t.TempDir()

		repoRemote, err := git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		emptyTreeHash, err := WriteTree(repoLocal, []object.TreeEntry{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Commit(repoLocal, emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}

		err = Push(context.Background(), repoLocal, remoteName, []string{refName})
		assert.Nil(t, err)

		refLocal, err := repoLocal.Reference(refNameTyped, true)
		if err != nil {
			t.Fatal(err)
		}
		refRemote, err := repoRemote.Reference(refNameTyped, true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, refLocal.Hash(), refRemote.Hash())
	})

	t.Run("assert no error when there are no updates to push", func(t *testing.T) {
		// The local repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote so we have a URL for it
		tmpDir := t.TempDir()

		_, err = git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		err = Push(context.Background(), repoLocal, remoteName, []string{refName})
		assert.Nil(t, err) // no error when it's already up to date
	})
}

func TestPushRefSpecRepository(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refSpecs := fmt.Sprintf("%s:%s", refName, refName)

	t.Run("assert remote repo does not have object until it is pushed", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		localTreeBuilder := NewReplacementTreeBuilder(localRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create an empty tree in the local repository
		emptyTreeHash, err := localTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}

		// Check that the empty tree is not present on the remote repository
		results, err := remoteRepo.GetAllFilesInTree(emptyTreeHash)
		assert.Nil(t, err)
		assert.Nil(t, results)

		if _, err := localRepo.Commit(emptyTreeHash, refName, "Test commit\n", false); err != nil {
			t.Fatal(err)
		}

		err = localRepo.PushRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)

		results, err = remoteRepo.GetAllFilesInTree(emptyTreeHash)
		assert.Nil(t, err)
		assert.NotNil(t, results)
	})

	t.Run("assert after push that src and dst refs match", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		localTreeBuilder := NewReplacementTreeBuilder(localRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create an empty tree in the local repository
		emptyTreeHash, err := localTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := localRepo.Commit(emptyTreeHash, refName, "Test commit\n", false); err != nil {
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

		localRepo := CreateTestGitRepository(t, localTmpDir)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.PushRefSpec(remoteName, []string{refSpecs})
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

		localRepo := CreateTestGitRepository(t, localTmpDir)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		localTreeBuilder := NewReplacementTreeBuilder(localRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create an empty tree in the local repository
		emptyTreeHash, err := localTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}

		// Check that the empty tree is not present on the remote repository
		results, err := remoteRepo.GetAllFilesInTree(emptyTreeHash)
		assert.Nil(t, err)
		assert.Nil(t, results)

		if _, err := localRepo.Commit(emptyTreeHash, refName, "Test commit\n", false); err != nil {
			t.Fatal(err)
		}

		err = localRepo.Push(remoteName, []string{refName})
		assert.Nil(t, err)

		results, err = remoteRepo.GetAllFilesInTree(emptyTreeHash)
		assert.Nil(t, err)
		assert.NotNil(t, results)
	})

	t.Run("assert after push that src and dst refs match", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		localTreeBuilder := NewReplacementTreeBuilder(localRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create an empty tree in the local repository
		emptyTreeHash, err := localTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}

		if _, err := localRepo.Commit(emptyTreeHash, refName, "Test commit\n", false); err != nil {
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

		localRepo := CreateTestGitRepository(t, localTmpDir)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.Push(remoteName, []string{refName})
		assert.Nil(t, err)
	})
}

func TestFetchRefSpec(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refSpecs := []config.RefSpec{config.RefSpec(fmt.Sprintf("+%s:%s", refName, refName))}
	refNameTyped := plumbing.ReferenceName(refName)

	t.Run("assert local repo does not have object until fetched", func(t *testing.T) {
		// The local repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := repoLocal.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, refNameTyped)); err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote repo so we have a URL for it
		tmpDir := t.TempDir()

		repoRemote, err := git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Check that the empty tree object we'll fetch later from the remote
		// repo is not present
		_, err = repoLocal.Object(plumbing.TreeObject, EmptyTree())
		assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)

		emptyTreeHash, err := WriteTree(repoRemote, []object.TreeEntry{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Commit(repoRemote, emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}

		err = FetchRefSpec(context.Background(), repoLocal, remoteName, refSpecs)
		assert.Nil(t, err)

		// This time, the empty tree object must also be in the local repo
		_, err = repoLocal.Object(plumbing.TreeObject, EmptyTree())
		assert.Nil(t, err)
	})

	t.Run("assert after fetch that both refs match", func(t *testing.T) {
		// The local repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := repoLocal.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, refNameTyped)); err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote repo so we have a URL for it
		tmpDir := t.TempDir()

		repoRemote, err := git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		emptyTreeHash, err := WriteTree(repoRemote, []object.TreeEntry{})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := Commit(repoRemote, emptyTreeHash, refName, "Test commit", false); err != nil {
			t.Fatal(err)
		}

		err = FetchRefSpec(context.Background(), repoLocal, remoteName, refSpecs)
		assert.Nil(t, err)

		refLocal, err := repoLocal.Reference(refNameTyped, true)
		if err != nil {
			t.Fatal(err)
		}
		refRemote, err := repoRemote.Reference(refNameTyped, true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, refRemote.Hash(), refLocal.Hash())
	})

	t.Run("assert no error when there are no updates to fetch", func(t *testing.T) {
		// The local repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := repoLocal.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, refNameTyped)); err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote repo so we have a URL for it
		tmpDir := t.TempDir()

		_, err = git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		err = FetchRefSpec(context.Background(), repoLocal, remoteName, refSpecs)
		assert.Nil(t, err)
	})
}

func TestFetch(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refNameTyped := plumbing.ReferenceName(refName)

	t.Run("assert local repo does not have object until fetched", func(t *testing.T) {
		// The local repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := repoLocal.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, refNameTyped)); err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote repo so we have a URL for it
		tmpDir := t.TempDir()

		repoRemote, err := git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		// Check that the empty tree object we'll fetch later from the remote
		// repo is not present
		_, err = repoLocal.Object(plumbing.TreeObject, EmptyTree())
		assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)

		emptyTreeHash, err := WriteTree(repoRemote, []object.TreeEntry{})
		if err != nil {
			t.Fatal(err)
		}
		remoteCommitID, err := Commit(repoRemote, emptyTreeHash, refName, "Test commit", false)
		if err != nil {
			t.Fatal(err)
		}

		err = Fetch(context.Background(), repoLocal, remoteName, []string{refName}, true)
		assert.Nil(t, err)

		// This time, the empty tree object must also be in the local repo
		_, err = repoLocal.Object(plumbing.TreeObject, EmptyTree())
		assert.Nil(t, err)

		assertLocalRefAndRemoteTrackerRef(t, repoLocal, refName, remoteName, remoteCommitID)
	})

	t.Run("assert after fetch that both refs match", func(t *testing.T) {
		// The local repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := repoLocal.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, refNameTyped)); err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote repo so we have a URL for it
		tmpDir := t.TempDir()

		repoRemote, err := git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		emptyTreeHash, err := WriteTree(repoRemote, []object.TreeEntry{})
		if err != nil {
			t.Fatal(err)
		}
		remoteCommitID, err := Commit(repoRemote, emptyTreeHash, refName, "Test commit", false)
		if err != nil {
			t.Fatal(err)
		}

		err = Fetch(context.Background(), repoLocal, remoteName, []string{refName}, true)
		assert.Nil(t, err)

		assertLocalRefAndRemoteTrackerRef(t, repoLocal, refName, remoteName, remoteCommitID)
	})

	t.Run("assert no error when there are no updates to fetch", func(t *testing.T) {
		// The local repo can be in-memory
		repoLocal, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := repoLocal.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, refNameTyped)); err != nil {
			t.Fatal(err)
		}

		// Create tmp dir for remote repo so we have a URL for it
		tmpDir := t.TempDir()

		_, err = git.PlainInit(tmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		_, err = repoLocal.CreateRemote(&config.RemoteConfig{
			Name: remoteName,
			URLs: []string{tmpDir},
		})
		if err != nil {
			t.Fatal(err)
		}

		err = Fetch(context.Background(), repoLocal, remoteName, []string{refName}, true)
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

		localRepo := CreateTestGitRepository(t, localTmpDir)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		remoteTreeBuilder := NewReplacementTreeBuilder(remoteRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create an empty tree in the remote repository
		emptyTreeHash, err := remoteTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}

		// Check that the empty tree is not present on the local repository
		results, err := localRepo.GetAllFilesInTree(emptyTreeHash)
		assert.Nil(t, err)
		assert.Nil(t, results)

		_, err = remoteRepo.Commit(emptyTreeHash, refName, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.FetchRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)

		_, err = localRepo.GetAllFilesInTree(emptyTreeHash)
		assert.Nil(t, err)
	})

	t.Run("assert after fetch that both refs match", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		remoteTreeBuilder := NewReplacementTreeBuilder(remoteRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		emptyTreeHash, err := remoteTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = remoteRepo.Commit(emptyTreeHash, refName, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.FetchRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)
	})

	t.Run("assert no error when there are no updates to fetch", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		if err := remoteRepo.SetReference(refName, ZeroHash); err != nil {
			t.Fatal(err)
		}

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.FetchRefSpec(remoteName, []string{refSpecs})
		assert.Nil(t, err)
	})
}

func TestFetchRepository(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"

	t.Run("assert local repo does not have object until fetched", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		remoteTreeBuilder := NewReplacementTreeBuilder(remoteRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		// Create an empty tree in the remote repository
		emptyTreeHash, err := remoteTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}

		// Check that the empty tree is not present on the local repository
		results, err := localRepo.GetAllFilesInTree(emptyTreeHash)
		assert.Nil(t, err)
		assert.Nil(t, results)

		_, err = remoteRepo.Commit(emptyTreeHash, refName, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.Fetch(remoteName, []string{refName}, true)
		assert.Nil(t, err)

		_, err = localRepo.GetAllFilesInTree(emptyTreeHash)
		assert.Nil(t, err)
	})

	t.Run("assert after fetch that both refs match", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		remoteTreeBuilder := NewReplacementTreeBuilder(remoteRepo)

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		emptyTreeHash, err := remoteTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}
		_, err = remoteRepo.Commit(emptyTreeHash, refName, "Test commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		err = localRepo.Fetch(remoteName, []string{refName}, true)
		assert.Nil(t, err)
	})

	t.Run("assert no error when there are no updates to fetch", func(t *testing.T) {
		// Create local and remote repositories
		localTmpDir := t.TempDir()
		remoteTmpDir := t.TempDir()

		localRepo := CreateTestGitRepository(t, localTmpDir)
		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		if err := remoteRepo.SetReference(refName, ZeroHash); err != nil {
			t.Fatal(err)
		}

		// Create the remote on the local repository
		if err := localRepo.CreateRemote(remoteName, remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		err := localRepo.Fetch(remoteName, []string{refName}, true)
		assert.Nil(t, err)
	})
}

func TestCloneAndFetchRepository(t *testing.T) {
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	t.Run("clone and fetch remote repository, verify refs match", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		remoteTreeBuilder := NewReplacementTreeBuilder(remoteRepo)

		emptyTreeHash, err := remoteTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}

		mainCommit, err := remoteRepo.Commit(emptyTreeHash, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}
		otherCommit, err := remoteRepo.Commit(emptyTreeHash, anotherRefName, "Commit to feature", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.SetReference("HEAD", mainCommit); err != nil {
			t.Fatal(err)
		}

		localRepo, err := CloneAndFetchRepository(remoteTmpDir, localTmpDir, refName, []string{anotherRefName})
		if err != nil {
			t.Fatal(err)
		}

		localMainCommit, err := localRepo.GetReference(refName)
		assert.Nil(t, err)
		localOtherCommit, err := localRepo.GetReference(anotherRefName)
		assert.Nil(t, err)

		assert.Equal(t, mainCommit, localMainCommit)
		assert.Equal(t, otherCommit, localOtherCommit)
	})

	t.Run("clone and fetch remote repository without specifying initial branch, verify refs match", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		remoteTreeBuilder := NewReplacementTreeBuilder(remoteRepo)

		emptyTreeHash, err := remoteTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}

		mainCommit, err := remoteRepo.Commit(emptyTreeHash, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}
		otherCommit, err := remoteRepo.Commit(emptyTreeHash, anotherRefName, "Commit to feature", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.SetReference("HEAD", mainCommit); err != nil {
			t.Fatal(err)
		}

		localRepo, err := CloneAndFetchRepository(remoteTmpDir, localTmpDir, "", []string{anotherRefName})
		if err != nil {
			t.Fatal(err)
		}

		localMainCommit, err := localRepo.GetReference(refName)
		assert.Nil(t, err)
		localOtherCommit, err := localRepo.GetReference(anotherRefName)
		assert.Nil(t, err)

		assert.Equal(t, mainCommit, localMainCommit)
		assert.Equal(t, otherCommit, localOtherCommit)
	})

	t.Run("clone and fetch remote repository with only one ref, verify refs match", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		remoteRepo := CreateTestGitRepository(t, remoteTmpDir)

		remoteTreeBuilder := NewReplacementTreeBuilder(remoteRepo)

		emptyTreeHash, err := remoteTreeBuilder.writeTree([]*entry{})
		if err != nil {
			t.Fatal(err)
		}

		mainCommit, err := remoteRepo.Commit(emptyTreeHash, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.SetReference("HEAD", mainCommit); err != nil {
			t.Fatal(err)
		}

		localRepo, err := CloneAndFetchRepository(remoteTmpDir, localTmpDir, "", []string{})
		if err != nil {
			t.Fatal(err)
		}

		localMainCommit, err := localRepo.GetReference(refName)
		assert.Nil(t, err)
		assert.Equal(t, mainCommit, localMainCommit)
	})
}

func TestCloneAndFetch(t *testing.T) {
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	t.Run("clone and fetch remote repository, verify refs match", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		// Create remote repo on disk so we can use its URL
		remoteRepo, err := git.PlainInit(remoteTmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate actions
		emptyTreeHash, err := WriteTree(remoteRepo, nil)
		if err != nil {
			t.Fatal(err)
		}
		mainCommitID, err := Commit(remoteRepo, emptyTreeHash, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}
		otherCommitID, err := Commit(remoteRepo, emptyTreeHash, anotherRefName, "Commit to feature", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.Storer.SetReference(plumbing.NewSymbolicReference("HEAD", plumbing.ReferenceName(refName))); err != nil {
			t.Fatal(err)
		}

		// Clone and fetch additional ref
		localRepo, err := CloneAndFetch(context.Background(), remoteTmpDir, localTmpDir, refName, []string{anotherRefName})
		if err != nil {
			t.Fatal(err)
		}

		localMainCommitID, err := localRepo.ResolveRevision(plumbing.Revision(refName))
		assert.Nil(t, err)
		localOtherCommitID, err := localRepo.ResolveRevision(plumbing.Revision(anotherRefName))
		assert.Nil(t, err)

		assert.Equal(t, mainCommitID, *localMainCommitID)
		assert.Equal(t, otherCommitID, *localOtherCommitID)
	})

	t.Run("clone and fetch remote repository without specifying initial branch, verify refs match", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		// Create remote repo on disk so we can use its URL
		remoteRepo, err := git.PlainInit(remoteTmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate actions
		emptyTreeHash, err := WriteTree(remoteRepo, nil)
		if err != nil {
			t.Fatal(err)
		}
		mainCommitID, err := Commit(remoteRepo, emptyTreeHash, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}
		otherCommitID, err := Commit(remoteRepo, emptyTreeHash, anotherRefName, "Commit to feature", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.Storer.SetReference(plumbing.NewSymbolicReference("HEAD", plumbing.ReferenceName(refName))); err != nil {
			t.Fatal(err)
		}

		// Clone and fetch additional ref
		localRepo, err := CloneAndFetch(context.Background(), remoteTmpDir, localTmpDir, "", []string{anotherRefName})
		if err != nil {
			t.Fatal(err)
		}

		localMainCommitID, err := localRepo.ResolveRevision(plumbing.Revision(refName))
		assert.Nil(t, err)
		localOtherCommitID, err := localRepo.ResolveRevision(plumbing.Revision(anotherRefName))
		assert.Nil(t, err)

		assert.Equal(t, mainCommitID, *localMainCommitID)
		assert.Equal(t, otherCommitID, *localOtherCommitID)
	})

	t.Run("clone and fetch remote repository with only one ref, verify refs match", func(t *testing.T) {
		remoteTmpDir := t.TempDir()
		localTmpDir := t.TempDir()

		// Create remote repo on disk so we can use its URL
		remoteRepo, err := git.PlainInit(remoteTmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate actions
		emptyTreeHash, err := WriteTree(remoteRepo, nil)
		if err != nil {
			t.Fatal(err)
		}
		mainCommitID, err := Commit(remoteRepo, emptyTreeHash, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.Storer.SetReference(plumbing.NewSymbolicReference("HEAD", plumbing.ReferenceName(refName))); err != nil {
			t.Fatal(err)
		}

		// Clone
		localRepo, err := CloneAndFetch(context.Background(), remoteTmpDir, localTmpDir, refName, nil)
		if err != nil {
			t.Fatal(err)
		}

		localMainCommitID, err := localRepo.ResolveRevision(plumbing.Revision(refName))
		assert.Nil(t, err)
		assert.Equal(t, mainCommitID, *localMainCommitID)
	})
}

func TestCloneAndFetchToMemory(t *testing.T) {
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"
	// refs := []config.RefSpec{config.RefSpec(fmt.Sprintf("%s:%s", anotherRefName, anotherRefName))}

	t.Run("clone and fetch remote repository, verify refs match", func(t *testing.T) {
		remoteTmpDir := t.TempDir()

		// Create remote repo on disk so we can use its URL
		remoteRepo, err := git.PlainInit(remoteTmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate actions
		emptyTreeHash, err := WriteTree(remoteRepo, nil)
		if err != nil {
			t.Fatal(err)
		}
		mainCommitID, err := Commit(remoteRepo, emptyTreeHash, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}
		otherCommitID, err := Commit(remoteRepo, emptyTreeHash, anotherRefName, "Commit to feature", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.Storer.SetReference(plumbing.NewSymbolicReference("HEAD", plumbing.ReferenceName(refName))); err != nil {
			t.Fatal(err)
		}

		// Clone and fetch additional ref
		localRepo, err := CloneAndFetchToMemory(context.Background(), remoteTmpDir, refName, []string{anotherRefName})
		if err != nil {
			t.Fatal(err)
		}

		localMainCommitID, err := localRepo.ResolveRevision(plumbing.Revision(refName))
		assert.Nil(t, err)
		localOtherCommitID, err := localRepo.ResolveRevision(plumbing.Revision(anotherRefName))
		assert.Nil(t, err)

		assert.Equal(t, mainCommitID, *localMainCommitID)
		assert.Equal(t, otherCommitID, *localOtherCommitID)
	})

	t.Run("clone and fetch remote repository without specifying initial branch, verify refs match", func(t *testing.T) {
		remoteTmpDir := t.TempDir()

		// Create remote repo on disk so we can use its URL
		remoteRepo, err := git.PlainInit(remoteTmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate actions
		emptyTreeHash, err := WriteTree(remoteRepo, nil)
		if err != nil {
			t.Fatal(err)
		}
		mainCommitID, err := Commit(remoteRepo, emptyTreeHash, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}
		otherCommitID, err := Commit(remoteRepo, emptyTreeHash, anotherRefName, "Commit to feature", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.Storer.SetReference(plumbing.NewSymbolicReference("HEAD", plumbing.ReferenceName(refName))); err != nil {
			t.Fatal(err)
		}

		// Clone and fetch additional ref
		localRepo, err := CloneAndFetchToMemory(context.Background(), remoteTmpDir, "", []string{anotherRefName})
		if err != nil {
			t.Fatal(err)
		}

		localMainCommitID, err := localRepo.ResolveRevision(plumbing.Revision(refName))
		assert.Nil(t, err)
		localOtherCommitID, err := localRepo.ResolveRevision(plumbing.Revision(anotherRefName))
		assert.Nil(t, err)

		assert.Equal(t, mainCommitID, *localMainCommitID)
		assert.Equal(t, otherCommitID, *localOtherCommitID)
	})

	t.Run("clone and fetch remote repository with only one ref, verify refs match", func(t *testing.T) {
		remoteTmpDir := t.TempDir()

		// Create remote repo on disk so we can use its URL
		remoteRepo, err := git.PlainInit(remoteTmpDir, true)
		if err != nil {
			t.Fatal(err)
		}

		// Simulate actions
		emptyTreeHash, err := WriteTree(remoteRepo, nil)
		if err != nil {
			t.Fatal(err)
		}
		mainCommitID, err := Commit(remoteRepo, emptyTreeHash, refName, "Commit to main", false)
		if err != nil {
			t.Fatal(err)
		}

		if err := remoteRepo.Storer.SetReference(plumbing.NewSymbolicReference("HEAD", plumbing.ReferenceName(refName))); err != nil {
			t.Fatal(err)
		}

		// Clone
		localRepo, err := CloneAndFetchToMemory(context.Background(), remoteTmpDir, refName, nil)
		if err != nil {
			t.Fatal(err)
		}

		localMainCommitID, err := localRepo.ResolveRevision(plumbing.Revision(refName))
		assert.Nil(t, err)
		assert.Equal(t, mainCommitID, *localMainCommitID)
	})
}

func assertLocalRefAndRemoteTrackerRef(t *testing.T, repo *git.Repository, refName, remoteName string, expectedCommitID plumbing.Hash) {
	t.Helper()

	refNameTyped := plumbing.ReferenceName(refName)
	refRemoteNameTyped := plumbing.ReferenceName(RemoteRef(refName, remoteName))
	localRef, err := repo.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedCommitID, localRef.Hash())

	localRemoteTrackerRef, err := repo.Reference(refRemoteNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedCommitID, localRemoteTrackerRef.Hash())
}
