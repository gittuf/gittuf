// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package propagation

import (
	"testing"

	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
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
	pathTreeID, err := downstreamRepo.GetPathIDInTree("upstream", downstreamRootTreeID)
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
