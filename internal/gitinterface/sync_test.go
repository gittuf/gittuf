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

	// The source repo can be in-memory
	repoSrc, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	// Create tmp dir for destination repo so we have a URL for it
	tmpDir, err := os.MkdirTemp("", "gittuf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	repoDst, err := git.PlainInit(tmpDir, true)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repoSrc.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{tmpDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	if err := repoSrc.Storer.SetReference(plumbing.NewHashReference(refNameTyped, plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	err = Push(repoSrc, remoteName, []config.RefSpec{refSpec})
	assert.ErrorIs(t, err, git.NoErrAlreadyUpToDate)

	// Check that the empty tree object we'll later push to the dest repo is not
	// present
	_, err = repoDst.Object(plumbing.TreeObject, EmptyTree())
	assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)

	emptyTreeHash, err := WriteTree(repoSrc, []object.TreeEntry{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Commit(repoSrc, emptyTreeHash, refName, "Test commit", false); err != nil {
		t.Fatal(err)
	}

	err = Push(repoSrc, remoteName, []config.RefSpec{refSpec})
	assert.Nil(t, err)

	refSrc, err := repoSrc.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	refDst, err := repoDst.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, refSrc.Hash(), refDst.Hash())

	// This time, the empty tree object must also be in the destination repo
	_, err = repoDst.Object(plumbing.TreeObject, EmptyTree())
	assert.Nil(t, err)
}

func TestFetch(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refSpec := config.RefSpec(fmt.Sprintf("%s:%s", refName, refName))
	refNameTyped := plumbing.ReferenceName(refName)

	// The source repo can be in-memory
	repoSrc, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	// Create tmp dir for destination repo so we have a URL for it
	tmpDir, err := os.MkdirTemp("", "gittuf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	repoDst, err := git.PlainInit(tmpDir, true)
	if err != nil {
		t.Fatal(err)
	}

	_, err = repoSrc.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{tmpDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	// if err := repoDst.Storer.SetReference(plumbing.NewHashReference(refNameTyped, plumbing.ZeroHash)); err != nil {
	// 	t.Fatal(err)
	// }

	// err = Fetch(repoSrc, remoteName, []config.RefSpec{refSpec})
	// assert.ErrorIs(t, err, git.NoErrAlreadyUpToDate)
	// FIXME: what's the expected handling of uninitialized refs?

	// Check that the empty tree object we'll later push to the dest repo is not
	// present
	_, err = repoSrc.Object(plumbing.TreeObject, EmptyTree())
	assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)

	if err := repoDst.Storer.SetReference(plumbing.NewHashReference(refNameTyped, plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	emptyTreeHash, err := WriteTree(repoDst, []object.TreeEntry{})
	if err != nil {
		t.Fatal(err)
	}
	if err := Commit(repoDst, emptyTreeHash, refName, "Test commit", false); err != nil {
		t.Fatal(err)
	}

	err = Fetch(repoSrc, remoteName, []config.RefSpec{refSpec})
	assert.Nil(t, err)

	refSrc, err := repoSrc.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	refDst, err := repoDst.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, refSrc.Hash(), refDst.Hash())

	// This time, the empty tree object must also be in the destination repo
	_, err = repoDst.Object(plumbing.TreeObject, EmptyTree())
	assert.Nil(t, err)
}
