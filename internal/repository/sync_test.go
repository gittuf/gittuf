package repository

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

func TestPush(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	refNameTyped := plumbing.ReferenceName(refName)
	rslRefNameTyped := plumbing.ReferenceName(rsl.RSLRef)

	repoLocal := createTestRepositoryWithPolicy(t)

	// Create tmp dir for destination repo so we have a URL for it
	tmpDir, err := os.MkdirTemp("", "gittuf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	repoRemote, err := git.PlainInit(tmpDir, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repoLocal.r.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{tmpDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assert that the remote repo does not contain the main branch or the RSL
	_, err = repoRemote.Reference(refNameTyped, true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	_, err = repoRemote.Reference(rslRefNameTyped, true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)

	// Create a test commit and its RSL entry
	emptyTreeHash, err := gitinterface.WriteTree(repoLocal.r, []object.TreeEntry{})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := gitinterface.Commit(repoLocal.r, emptyTreeHash, refName, "Test commit", false); err != nil {
		t.Fatal(err)
	}

	if err := repoLocal.RecordRSLEntryForReference(refName, false); err != nil {
		t.Fatal(err)
	}

	// RSL is not explicitly named here for Push
	err = repoLocal.Push(remoteName, refName)
	assert.Nil(t, err)

	localRef, err := repoLocal.r.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	remoteRef, err := repoRemote.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, localRef.Hash(), remoteRef.Hash())

	localRSLRef, err := repoLocal.r.Reference(rslRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	remoteRSLRef, err := repoRemote.Reference(rslRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, localRSLRef.Hash(), remoteRSLRef.Hash())
}
