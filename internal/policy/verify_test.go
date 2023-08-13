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

	refName := "refs/heads/main"

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2)

	for _, commitID := range commitIDs {
		entry := rsl.NewCompleteEntry(refName, commitID)
		entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
		entry.ID = entryID

		err := VerifyRef(context.Background(), repo, refName)
		assert.Nil(t, err)
	}
}

func TestVerifyRefFull(t *testing.T) {
	// FIXME: currently this test is identical to the one for VerifyRef.
	// This is because it's not trivial to create a bunch of test policy / RSL
	// states cleanly. We need something that is easy to maintain and add cases
	// to.
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	refName := "refs/heads/main"

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2)

	for _, commitID := range commitIDs {
		entry := rsl.NewCompleteEntry(refName, commitID)
		entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
		entry.ID = entryID

		err := VerifyRefFull(context.Background(), repo, refName)
		assert.Nil(t, err)
	}
}

func TestVerifyRelativeForRef(t *testing.T) {
	// FIXME: currently this test is nearly identical to the one for VerifyRef.
	// This is because it's not trivial to create a bunch of test policy / RSL
	// states cleanly. We need something that is easy to maintain and add cases
	// to.
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	refName := "refs/heads/main"

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	policyEntry, err := rsl.GetLatestEntryForRef(repo, PolicyRef)
	if err != nil {
		t.Fatal(err)
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2)
	entries := []*rsl.Entry{}

	for _, commitID := range commitIDs {
		entry := rsl.NewCompleteEntry(refName, commitID)
		entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
		entry.ID = entryID

		entries = append(entries, entry)
	}

	err = VerifyRelativeForRef(context.Background(), repo, policyEntry, policyEntry, entries[0], refName)
	assert.Nil(t, err)

	err = VerifyRelativeForRef(context.Background(), repo, policyEntry, entries[0], entries[1], refName)
	assert.Nil(t, err)

	err = VerifyRelativeForRef(context.Background(), repo, policyEntry, entries[1], entries[0], refName)
	assert.ErrorIs(t, err, ErrNotAncestor)
}

func TestVerifyEntry(t *testing.T) {
	// FIXME: currently this test is nearly identical to the one for VerifyRef.
	// This is because it's not trivial to create a bunch of test policy / RSL
	// states cleanly. We need something that is easy to maintain and add cases
	// to.
	repo, state := createTestRepository(t, createTestStateWithPolicy)

	refName := "refs/heads/main"
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2)

	for _, commitID := range commitIDs {
		entry := rsl.NewCompleteEntry(refName, commitID)
		entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
		entry.ID = entryID

		err := verifyEntries(context.Background(), repo, state, []*rsl.Entry{entry})
		assert.Nil(t, err)
	}
}
