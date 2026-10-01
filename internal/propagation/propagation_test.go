// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package propagation

import (
	"errors"
	"testing"

	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
)

func TestPropagateChangesFromUpstreamRepository(t *testing.T) {
	// Create upstreamRepo
	upstreamRepoLocation := t.TempDir()
	upstreamRepo := gitinterface.CreateTestGitRepository(t, upstreamRepoLocation, true)

	downstreamRepoLocation := t.TempDir()
	downstreamRepo := gitinterface.CreateTestGitRepository(t, downstreamRepoLocation, true)

	propagationDetails := &tufv01.PropagationDirective{
		UpstreamReference:   "refs/heads/main",
		UpstreamRepository:  upstreamRepoLocation,
		DownstreamReference: "refs/heads/main",
		DownstreamPath:      "upstream",
	}

	err := PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{propagationDetails}, false)
	assert.Nil(t, err) // propagation has nothing to do because no RSL exists in upstream

	// Add things to upstreamRepo
	blobAID, err := upstreamRepo.WriteBlob([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}

	blobBID, err := upstreamRepo.WriteBlob([]byte("b"))
	if err != nil {
		t.Fatal(err)
	}

	upstreamTreeBuilder := gitinterface.NewTreeBuilder(upstreamRepo)
	upstreamRootTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
		gitinterface.NewEntryBlob("a", blobAID),
		gitinterface.NewEntryBlob("b", blobBID),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstreamCommitID, err := upstreamRepo.Commit(upstreamRootTreeID, "refs/heads/main", "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsl.NewReferenceEntry("refs/heads/main", upstreamCommitID).Commit(upstreamRepo, false); err != nil {
		t.Fatal(err)
	}
	upstreamEntry, err := rsl.GetLatestEntry(upstreamRepo)
	if err != nil {
		t.Fatal(err)
	}

	err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{propagationDetails}, false)
	// TODO: should propagation result in a new local ref?
	assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)

	// Add things to downstreamRepo
	blobAID, err = downstreamRepo.WriteBlob([]byte("a"))
	if err != nil {
		t.Fatal(err)
	}

	blobBID, err = downstreamRepo.WriteBlob([]byte("b"))
	if err != nil {
		t.Fatal(err)
	}

	downstreamTreeBuilder := gitinterface.NewTreeBuilder(downstreamRepo)
	downstreamRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
		gitinterface.NewEntryBlob("a", blobAID),
		gitinterface.NewEntryBlob("foo/b", blobBID),
	})
	if err != nil {
		t.Fatal(err)
	}
	downstreamCommitID, err := downstreamRepo.Commit(downstreamRootTreeID, "refs/heads/main", "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsl.NewReferenceEntry("refs/heads/main", downstreamCommitID).Commit(downstreamRepo, false); err != nil {
		t.Fatal(err)
	}

	err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{propagationDetails}, false)
	assert.Nil(t, err)

	latestEntry, err := rsl.GetLatestEntry(downstreamRepo)
	if err != nil {
		t.Fatal(err)
	}
	propagationEntry, isPropagationEntry := latestEntry.(*rsl.PropagationEntry)
	if !isPropagationEntry {
		t.Fatal("unexpected entry type in downstream repo")
	}
	assert.Equal(t, upstreamRepoLocation, propagationEntry.UpstreamRepository)
	assert.Equal(t, upstreamEntry.GetID(), propagationEntry.UpstreamEntryID)

	downstreamRootTreeID, err = downstreamRepo.GetCommitTreeID(propagationEntry.TargetID)
	if err != nil {
		t.Fatal(err)
	}
	pathTreeID, err := downstreamRepo.GetPathIDInTree(downstreamRootTreeID, "upstream")
	if err != nil {
		t.Fatal(err)
	}

	// Check the subtree ID in downstream repo matches upstream root tree ID
	assert.Equal(t, upstreamRootTreeID, pathTreeID)

	// Check the downstream tree still contains other items
	expectedRootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
		gitinterface.NewEntryBlob("a", blobAID),
		gitinterface.NewEntryBlob("foo/b", blobBID),
		gitinterface.NewEntryBlob("upstream/a", blobAID),
		gitinterface.NewEntryBlob("upstream/b", blobBID),
	})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedRootTreeID, downstreamRootTreeID)

	// Nothing to propagate, check that a new entry has not been added in the downstreamRepo
	err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{propagationDetails}, false)
	assert.Nil(t, err)

	latestEntry, err = rsl.GetLatestEntry(downstreamRepo)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, propagationEntry.GetID(), latestEntry.GetID())
}

func TestPropagateChangesCreateSubtreeError(t *testing.T) {
	// Upstream repo with a commit and RSL entry for "refs/heads/main".
	upstreamRepoLocation := t.TempDir()
	upstreamRepo := gitinterface.CreateTestGitRepository(t, upstreamRepoLocation, true)

	blobID, err := upstreamRepo.WriteBlob([]byte("content"))
	if err != nil {
		t.Fatal(err)
	}

	upstreamTreeBuilder := gitinterface.NewTreeBuilder(upstreamRepo)
	upstreamTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
		gitinterface.NewEntryBlob("file.txt", blobID),
	})
	if err != nil {
		t.Fatal(err)
	}

	upstreamCommitID, err := upstreamRepo.Commit(upstreamTreeID, "refs/heads/main", "Upstream commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsl.NewReferenceEntry("refs/heads/main", upstreamCommitID).Commit(upstreamRepo, false); err != nil {
		t.Fatal(err)
	}

	downstreamRepoLocation := t.TempDir()
	downstreamRepo := gitinterface.CreateTestGitRepository(t, downstreamRepoLocation, true)

	downstreamTreeBuilder := gitinterface.NewTreeBuilder(downstreamRepo)
	downstreamTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	downstreamCommitID, err := downstreamRepo.Commit(downstreamTreeID, "refs/heads/main", "Downstream commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsl.NewReferenceEntry("refs/heads/main", downstreamCommitID).Commit(downstreamRepo, false); err != nil {
		t.Fatal(err)
	}

	directive := &tufv01.PropagationDirective{
		UpstreamReference:   "refs/heads/main",
		UpstreamRepository:  upstreamRepoLocation,
		DownstreamReference: "refs/heads/main",
		DownstreamPath:      "", // empty path triggers ErrCannotCreateSubtreeIntoRootTree
	}

	err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{directive}, false)
	assert.ErrorIs(t, err, gitinterface.ErrCannotCreateSubtreeIntoRootTree)
}

func TestPropagateChangesUpstreamPathMissing(t *testing.T) {
	upstreamRepoLocation := t.TempDir()
	upstreamRepo := gitinterface.CreateTestGitRepository(t, upstreamRepoLocation, true)

	blobID, err := upstreamRepo.WriteBlob([]byte("content"))
	if err != nil {
		t.Fatal(err)
	}

	upstreamTreeBuilder := gitinterface.NewTreeBuilder(upstreamRepo)
	upstreamTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
		gitinterface.NewEntryBlob("file.txt", blobID),
	})
	if err != nil {
		t.Fatal(err)
	}

	upstreamCommitID, err := upstreamRepo.Commit(upstreamTreeID, "refs/heads/main", "Upstream commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsl.NewReferenceEntry("refs/heads/main", upstreamCommitID).Commit(upstreamRepo, false); err != nil {
		t.Fatal(err)
	}

	downstreamRepoLocation := t.TempDir()
	downstreamRepo := gitinterface.CreateTestGitRepository(t, downstreamRepoLocation, true)

	downstreamTreeBuilder := gitinterface.NewTreeBuilder(downstreamRepo)
	downstreamTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}

	downstreamCommitID, err := downstreamRepo.Commit(downstreamTreeID, "refs/heads/main", "Downstream commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsl.NewReferenceEntry("refs/heads/main", downstreamCommitID).Commit(downstreamRepo, false); err != nil {
		t.Fatal(err)
	}

	directive := &tufv01.PropagationDirective{
		UpstreamReference:   "refs/heads/main",
		UpstreamRepository:  upstreamRepoLocation,
		UpstreamPath:        "nonexistent-dir",
		DownstreamReference: "refs/heads/main",
		DownstreamPath:      "upstream",
	}

	err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{directive}, false)
	assert.ErrorIs(t, err, gitinterface.ErrTreeDoesNotHavePath)
}

type mockDownstreamRepo struct {
	*gitinterface.Repository
	getCommitTreeIDFunc func(githash.Hash) (githash.Hash, error)
	getPathIDInTreeFunc func(githash.Hash, string) (githash.Hash, error)
	commitFunc          func(githash.Hash, string, string, bool) (githash.Hash, error)
}

func (m *mockDownstreamRepo) GetCommitTreeID(commitID githash.Hash) (githash.Hash, error) {
	if m.getCommitTreeIDFunc != nil {
		return m.getCommitTreeIDFunc(commitID)
	}
	return m.Repository.GetCommitTreeID(commitID)
}

func (m *mockDownstreamRepo) GetPathIDInTree(treeID githash.Hash, treePath string) (githash.Hash, error) {
	if m.getPathIDInTreeFunc != nil {
		return m.getPathIDInTreeFunc(treeID, treePath)
	}
	return m.Repository.GetPathIDInTree(treeID, treePath)
}

func (m *mockDownstreamRepo) Commit(treeID githash.Hash, targetRef, message string, sign bool) (githash.Hash, error) {
	if m.commitFunc != nil {
		return m.commitFunc(treeID, targetRef, message, sign)
	}
	return m.Repository.Commit(treeID, targetRef, message, sign)
}
func TestPropagateChanges_MockErrors(t *testing.T) {
	upstreamRepoLocation := t.TempDir()
	upstreamRepo := gitinterface.CreateTestGitRepository(t, upstreamRepoLocation, true)

	blobID, err := upstreamRepo.WriteBlob([]byte("content"))
	if err != nil {
		t.Fatal(err)
	}
	upstreamTreeBuilder := gitinterface.NewTreeBuilder(upstreamRepo)
	upstreamTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
		gitinterface.NewEntryBlob("file.txt", blobID),
	})
	if err != nil {
		t.Fatal(err)
	}
	upstreamCommitID, err := upstreamRepo.Commit(upstreamTreeID, "refs/heads/main", "Upstream commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsl.NewReferenceEntry("refs/heads/main", upstreamCommitID).Commit(upstreamRepo, false); err != nil {
		t.Fatal(err)
	}

	downstreamRepoLocation := t.TempDir()
	downstreamRepoBase := gitinterface.CreateTestGitRepository(t, downstreamRepoLocation, true)
	downstreamTreeBuilder := gitinterface.NewTreeBuilder(downstreamRepoBase)
	downstreamTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries(nil)
	if err != nil {
		t.Fatal(err)
	}
	downstreamCommitID, err := downstreamRepoBase.Commit(downstreamTreeID, "refs/heads/main", "Downstream commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := rsl.NewReferenceEntry("refs/heads/main", downstreamCommitID).Commit(downstreamRepoBase, false); err != nil {
		t.Fatal(err)
	}

	directive := &tufv01.PropagationDirective{
		UpstreamReference:   "refs/heads/main",
		UpstreamRepository:  upstreamRepoLocation,
		DownstreamReference: "refs/heads/main",
		DownstreamPath:      "upstream",
	}

	t.Run("GetCommitTreeID returns error", func(t *testing.T) {
		mockRepo := &mockDownstreamRepo{
			Repository: downstreamRepoBase,
			getCommitTreeIDFunc: func(_ githash.Hash) (githash.Hash, error) {
				return gitinterface.ZeroHash, errors.New("mock GetCommitTreeID error")
			},
		}
		err := PropagateChangesFromUpstreamRepository(mockRepo, upstreamRepo, []tuf.PropagationDirective{directive}, false)
		assert.ErrorContains(t, err, "mock GetCommitTreeID error")
	})

	t.Run("GetPathIDInTree returns non-ErrTreeDoesNotHavePath error", func(t *testing.T) {
		mockRepo := &mockDownstreamRepo{
			Repository: downstreamRepoBase,
			getPathIDInTreeFunc: func(_ githash.Hash, _ string) (githash.Hash, error) {
				return gitinterface.ZeroHash, errors.New("mock GetPathIDInTree error")
			},
		}
		err := PropagateChangesFromUpstreamRepository(mockRepo, upstreamRepo, []tuf.PropagationDirective{directive}, false)
		assert.ErrorContains(t, err, "mock GetPathIDInTree error")
	})

	t.Run("rsl.Commit returns error", func(t *testing.T) {
		mockRepo := &mockDownstreamRepo{
			Repository: downstreamRepoBase,
			commitFunc: func(treeID githash.Hash, targetRef, message string, sign bool) (githash.Hash, error) {
				if targetRef == rsl.Ref {
					return gitinterface.ZeroHash, errors.New("mock RSL Commit error")
				}
				return downstreamRepoBase.Commit(treeID, targetRef, message, sign)
			},
		}
		err := PropagateChangesFromUpstreamRepository(mockRepo, upstreamRepo, []tuf.PropagationDirective{directive}, false)
		assert.ErrorContains(t, err, "mock RSL Commit error")
	})
}
func TestPropagateChanges_UpstreamErrors(t *testing.T) {
	downstreamRepoLocation := t.TempDir()
	downstreamRepo := gitinterface.CreateTestGitRepository(t, downstreamRepoLocation, true)

	t.Run("GetLatestReferenceUpdaterEntry returns non-NotFound error", func(t *testing.T) {
		upstreamRepoLocation := t.TempDir()
		upstreamRepo := gitinterface.CreateTestGitRepository(t, upstreamRepoLocation, true)

		// Create a bad RSL entry (invalid JSON message)
		blobID, _ := upstreamRepo.WriteBlob([]byte("content"))
		treeBuilder := gitinterface.NewTreeBuilder(upstreamRepo)
		treeID, _ := treeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{
			gitinterface.NewEntryBlob("file", blobID),
		})
		_, err := upstreamRepo.Commit(treeID, rsl.Ref, "not-json-message", false)
		assert.NoError(t, err)

		directive := &tufv01.PropagationDirective{
			UpstreamReference:   "refs/heads/main",
			UpstreamRepository:  upstreamRepoLocation,
			DownstreamReference: "refs/heads/main",
			DownstreamPath:      "upstream",
		}
		err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{directive}, false)
		assert.Error(t, err)
		assert.NotErrorIs(t, err, rsl.ErrRSLEntryNotFound)
	})

	t.Run("GetCommitTreeID fails for upstream target", func(t *testing.T) {
		upstreamRepoLocation := t.TempDir()
		upstreamRepo := gitinterface.CreateTestGitRepository(t, upstreamRepoLocation, true)

		blobID, _ := upstreamRepo.WriteBlob([]byte("content"))
		// RSL points to a blob, not a commit, so GetCommitTreeID will fail
		err := rsl.NewReferenceEntry("refs/heads/main", blobID).Commit(upstreamRepo, false)
		assert.NoError(t, err)

		// Downstream repo must have some reference so it gets past the downstream checks
		downstreamTreeBuilder := gitinterface.NewTreeBuilder(downstreamRepo)
		downstreamTreeID, _ := downstreamTreeBuilder.WriteTreeFromEntries(nil)
		downstreamCommitID, _ := downstreamRepo.Commit(downstreamTreeID, "refs/heads/main", "Downstream commit\n", false)
		_ = rsl.NewReferenceEntry("refs/heads/main", downstreamCommitID).Commit(downstreamRepo, false)

		directive := &tufv01.PropagationDirective{
			UpstreamReference:   "refs/heads/main",
			UpstreamRepository:  upstreamRepoLocation,
			DownstreamReference: "refs/heads/main",
			DownstreamPath:      "upstream", // wait, if downstream doesn't have 'upstream' path it will continue
		}
		err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{directive}, false)
		assert.Error(t, err)
	})
}
