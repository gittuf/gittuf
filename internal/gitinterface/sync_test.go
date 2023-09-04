package gitinterface

import (
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

func TestPush(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refSpec := config.RefSpec(fmt.Sprintf("%s:%s", refName, refName))
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

		err = Push(repoLocal, remoteName, []config.RefSpec{refSpec})
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

		err = Push(repoLocal, remoteName, []config.RefSpec{refSpec})
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

		err = Push(repoLocal, remoteName, []config.RefSpec{refSpec})
		assert.Nil(t, err) // no error when it's already up to date
	})
}

func TestFetch(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refSpec := config.RefSpec(fmt.Sprintf("%s:%s", refName, refName))
	refNameTyped := plumbing.ReferenceName(refName)

	t.Run("assert local repo does not have object until fetched", func(t *testing.T) {
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

		err = Fetch(repoLocal, remoteName, []config.RefSpec{refSpec})
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

		err = Fetch(repoLocal, remoteName, []config.RefSpec{refSpec})
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

		err = Fetch(repoLocal, remoteName, []config.RefSpec{refSpec})
		assert.Nil(t, err)
	})
}
