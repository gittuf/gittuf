package policy

import (
	"context"
	"testing"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestVerifyRef(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	entry := rsl.NewEntry("refs/heads/main", plumbing.ZeroHash)
	common.CreateTestRSLEntryCommit(t, repo, entry)

	err := VerifyRef(context.Background(), repo, "refs/heads/main")
	assert.Nil(t, err)
}

func TestVerifyRefFull(t *testing.T) {
	// FIXME: currently this test is identical to the one for VerifyRef.
	// This is because it's not trivial to create a bunch of test policy / RSL
	// states cleanly. We need something that is easy to maintain and add cases
	// to.
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	entry := rsl.NewEntry("refs/heads/main", plumbing.ZeroHash)
	common.CreateTestRSLEntryCommit(t, repo, entry)

	err := VerifyRefFull(context.Background(), repo, "refs/heads/main")
	assert.Nil(t, err)
}

func TestVerifyRelativeForRef(t *testing.T) {
	// FIXME: currently this test is nearly identical to the one for VerifyRef.
	// This is because it's not trivial to create a bunch of test policy / RSL
	// states cleanly. We need something that is easy to maintain and add cases
	// to.
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	policyEntry, err := rsl.GetLatestEntryForRef(repo, PolicyRef)
	if err != nil {
		t.Fatal(err)
	}

	entry := rsl.NewEntry("refs/heads/main", plumbing.ZeroHash)
	entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
	entry.ID = entryID

	err = VerifyRelativeForRef(context.Background(), repo, policyEntry, policyEntry, entry, "refs/heads/main")
	assert.Nil(t, err)

	err = VerifyRelativeForRef(context.Background(), repo, policyEntry, entry, policyEntry, "refs/heads/main")
	assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
}

func TestVerifyEntry(t *testing.T) {
	// FIXME: currently this test is nearly identical to the one for VerifyRef.
	// This is because it's not trivial to create a bunch of test policy / RSL
	// states cleanly. We need something that is easy to maintain and add cases
	// to.
	repo, state := createTestRepository(t, createTestStateWithPolicy)

	entry := rsl.NewEntry("refs/heads/main", plumbing.ZeroHash)
	entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
	entry.ID = entryID

	err := verifyEntry(context.Background(), repo, state, entry)
	assert.Nil(t, err)
}
