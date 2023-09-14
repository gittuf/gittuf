package gitinterface

import (
	"context"
	"fmt"
	"os"
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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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

		err = Fetch(context.Background(), repoLocal, remoteName, []string{refName}, false)
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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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

		err = Fetch(context.Background(), repoLocal, remoteName, []string{refName}, false)
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
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(tmpDir) //nolint:errcheck

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

		err = Fetch(context.Background(), repoLocal, remoteName, []string{refName}, false)
		assert.Nil(t, err)
	})
}

func TestCloneAndFetch(t *testing.T) {
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	t.Run("clone and fetch remote repository, verify refs match", func(t *testing.T) {
		remoteTmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(remoteTmpDir) //nolint:errcheck

		localTmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(localTmpDir) //nolint:errcheck

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
		localRepo, err := CloneAndFetch(context.Background(), remoteTmpDir, localTmpDir, refName, []string{anotherRefName}, false)
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
		remoteTmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(remoteTmpDir) //nolint:errcheck

		localTmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(localTmpDir) //nolint:errcheck

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
		localRepo, err := CloneAndFetch(context.Background(), remoteTmpDir, localTmpDir, "", []string{anotherRefName}, false)
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
		remoteTmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(remoteTmpDir) //nolint:errcheck

		localTmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(localTmpDir) //nolint:errcheck

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
		localRepo, err := CloneAndFetch(context.Background(), remoteTmpDir, localTmpDir, refName, nil, false)
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
		remoteTmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(remoteTmpDir) //nolint:errcheck

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
		localRepo, err := CloneAndFetchToMemory(context.Background(), remoteTmpDir, refName, []string{anotherRefName}, false)
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
		remoteTmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(remoteTmpDir) //nolint:errcheck

		localTmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(localTmpDir) //nolint:errcheck

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
		localRepo, err := CloneAndFetchToMemory(context.Background(), remoteTmpDir, "", []string{anotherRefName}, false)
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
		remoteTmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.RemoveAll(remoteTmpDir) //nolint:errcheck

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
		localRepo, err := CloneAndFetchToMemory(context.Background(), remoteTmpDir, refName, nil, false)
		if err != nil {
			t.Fatal(err)
		}

		localMainCommitID, err := localRepo.ResolveRevision(plumbing.Revision(refName))
		assert.Nil(t, err)
		assert.Equal(t, mainCommitID, *localMainCommitID)
	})
}
