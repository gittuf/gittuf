// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"errors"
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// overrideStorer wraps a real Storer, injecting failures for the methods the
// tests need to see fail while delegating everything else.
type overrideStorer struct {
	gitstore.Storer

	emptyTreeErr        error
	getReferenceErr     error
	getCommitMessageErr error
}

func (o *overrideStorer) EmptyTree() (Hash, error) {
	if o.emptyTreeErr != nil {
		return nil, o.emptyTreeErr
	}
	return o.Storer.EmptyTree()
}

func (o *overrideStorer) GetReference(refName string) (Hash, error) {
	if o.getReferenceErr != nil {
		return nil, o.getReferenceErr
	}
	return o.Storer.GetReference(refName)
}

func (o *overrideStorer) GetCommitMessage(commitID Hash) (string, error) {
	if o.getCommitMessageErr != nil {
		return "", o.getCommitMessageErr
	}
	return o.Storer.GetCommitMessage(commitID)
}

func TestReferenceEntryCommitStorerErrors(t *testing.T) {
	t.Parallel()

	t.Run("empty tree error", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		injected := errors.New("empty tree failure")
		storer := &overrideStorer{Storer: repo, emptyTreeErr: injected}

		err := NewReferenceEntry("refs/heads/empty-tree-error", gitinterface.ZeroHash).Commit(storer, false)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("empty tree error using specific key", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		injected := errors.New("empty tree failure")
		storer := &overrideStorer{Storer: repo, emptyTreeErr: injected}

		err := NewReferenceEntry("refs/heads/empty-tree-error-key", gitinterface.ZeroHash).CommitUsingSpecificKey(storer, artifacts.SSHED25519Private)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("get reference error", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		injected := errors.New("get reference failure")
		storer := &overrideStorer{Storer: repo, getReferenceErr: injected}

		err := NewReferenceEntry("refs/heads/get-reference-error", gitinterface.ZeroHash).Commit(storer, false)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("get reference error using specific key", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		injected := errors.New("get reference failure")
		storer := &overrideStorer{Storer: repo, getReferenceErr: injected}

		err := NewReferenceEntry("refs/heads/get-reference-error-key", gitinterface.ZeroHash).CommitUsingSpecificKey(storer, artifacts.SSHED25519Private)
		assert.ErrorIs(t, err, injected)
	})
}

func TestAnnotationEntryCommitStorerErrors(t *testing.T) {
	t.Parallel()

	t.Run("get reference error", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		injected := errors.New("get reference failure")
		storer := &overrideStorer{Storer: repo, getReferenceErr: injected}

		err := NewAnnotationEntry(nil, false, annotationMessage).Commit(storer, false)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("get reference error using specific key", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		injected := errors.New("get reference failure")
		storer := &overrideStorer{Storer: repo, getReferenceErr: injected}

		err := NewAnnotationEntry(nil, false, annotationMessage).CommitUsingSpecificKey(storer, artifacts.SSHED25519Private)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("referenced entry lookup error", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		require.Nil(t, NewReferenceEntry("refs/heads/annotation-lookup-error", gitinterface.ZeroHash).Commit(repo, false))
		entryID, err := repo.GetReference(Ref)
		require.Nil(t, err)

		injected := errors.New("get commit message failure")
		storer := &overrideStorer{Storer: repo, getCommitMessageErr: injected}

		err = NewAnnotationEntry([]Hash{entryID}, true, annotationMessage).Commit(storer, false)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("referenced entry lookup error using specific key", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		require.Nil(t, NewReferenceEntry("refs/heads/annotation-lookup-error-key", gitinterface.ZeroHash).Commit(repo, false))
		entryID, err := repo.GetReference(Ref)
		require.Nil(t, err)

		injected := errors.New("get commit message failure")
		storer := &overrideStorer{Storer: repo, getCommitMessageErr: injected}

		err = NewAnnotationEntry([]Hash{entryID}, true, annotationMessage).CommitUsingSpecificKey(storer, artifacts.SSHED25519Private)
		assert.ErrorIs(t, err, injected)
	})
}
