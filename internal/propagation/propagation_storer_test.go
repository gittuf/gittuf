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

// TestPropagateChangesCreateSubtreeError covers the error branch (lines 65-67)
// in PropagateChangesFromUpstreamRepository where CreateSubtreeFromUpstreamRepository
// returns an error.
//
// The directive specifies an empty DownstreamPath, which causes
// CreateSubtreeFromUpstreamRepository to return ErrCannotCreateSubtreeIntoRootTree
// immediately (before any remote access), exercising the return-on-error path.
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

	// Downstream repo with a commit on "refs/heads/main" so GetReference and
	// GetCommitTreeID both succeed, reaching CreateSubtreeFromUpstreamRepository.
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

	// Directive with an empty DownstreamPath. CreateSubtreeFromUpstreamRepository
	// returns ErrCannotCreateSubtreeIntoRootTree when the local path is empty,
	// which triggers the error branch at propagation.go line 65-67.
	directive := &tufv01.PropagationDirective{
		UpstreamReference:   "refs/heads/main",
		UpstreamRepository:  upstreamRepoLocation,
		DownstreamReference: "refs/heads/main",
		DownstreamPath:      "", // empty path triggers ErrCannotCreateSubtreeIntoRootTree
	}

	err = PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo, []tuf.PropagationDirective{directive}, false)
	assert.ErrorIs(t, err, gitinterface.ErrCannotCreateSubtreeIntoRootTree)
}

// TestPropagateChangesUpstreamPathMissing covers lines 65-67 via a directive
// whose UpstreamPath does not exist in the upstream commit tree, causing
// CreateSubtreeFromUpstreamRepository to return ErrTreeDoesNotHavePath.
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

	// The upstream tree only has "file.txt". Requesting UpstreamPath "nonexistent-dir"
	// causes CreateSubtreeFromUpstreamRepository to call GetPathIDInTree, which
	// returns ErrTreeDoesNotHavePath, exercising lines 65-67.
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
