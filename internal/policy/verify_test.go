package policy

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestVerifyRef(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	// FIXME: In a future change, this will be updated to use
	// common.AddNTestCommitsToSpecifiedRef. For that to happen, the
	// verification must also check file policies.
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

	// FIXME: In a future change, this will be updated to use
	// common.AddNTestCommitsToSpecifiedRef. For that to happen, the
	// verification must also check file policies.
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

	// FIXME: In a future change, this will be updated to use
	// common.AddNTestCommitsToSpecifiedRef. For that to happen, the
	// verification must also check file policies.
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

	// FIXME: In a future change, this will be updated to use
	// common.AddNTestCommitsToSpecifiedRef. For that to happen, the
	// verification must also check file policies.
	entry := rsl.NewEntry("refs/heads/main", plumbing.ZeroHash)
	entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
	entry.ID = entryID

	err := verifyEntry(context.Background(), repo, state, entry)
	assert.Nil(t, err)
}

func TestGetChangedPaths(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	refName := "refs/heads/main"

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	// FIXME: this setup with RSL entries can be formalized using another
	// helper like createTestStateWithPolicy. The RSL could then also
	// incorporate policy changes and so on.
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2)
	entries := []*rsl.Entry{}
	for _, commitID := range commitIDs {
		entry := rsl.NewEntry(refName, commitID)
		entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
		entry.ID = entryID

		entries = append(entries, entry)
	}

	changedPaths, err := getChangedPaths(repo, entries[0])
	if err != nil {
		t.Fatal(err)
	}
	// First commit's tree has a single file, 1.
	assert.Equal(t, []string{"1"}, changedPaths)

	changedPaths, err = getChangedPaths(repo, entries[1])
	if err != nil {
		t.Fatal(err)
	}
	// Second commit's tree has two files, 1 and 2. Only 2 is new.
	assert.Equal(t, []string{"2"}, changedPaths)
}
