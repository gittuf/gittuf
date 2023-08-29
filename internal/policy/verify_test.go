package policy

import (
	"context"
	"sort"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/stretchr/testify/assert"
)

// FIXME: the verification tests do not check for expected failures. More
// broadly, we need to rework the test setup here starting with
// createTestRepository and the state creation helpers.

func TestVerifyRef(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1)
	entry := rsl.NewEntry(refName, commitIDs[0])
	common.CreateTestRSLEntryCommit(t, repo, entry)

	err := VerifyRef(context.Background(), repo, refName)
	assert.Nil(t, err)
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

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1)
	entry := rsl.NewEntry(refName, commitIDs[0])
	common.CreateTestRSLEntryCommit(t, repo, entry)

	err := VerifyRefFull(context.Background(), repo, refName)
	assert.Nil(t, err)
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

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1)
	entry := rsl.NewEntry(refName, commitIDs[0])
	entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
	entry.ID = entryID

	err = VerifyRelativeForRef(context.Background(), repo, policyEntry, policyEntry, entry, refName)
	assert.Nil(t, err)

	err = VerifyRelativeForRef(context.Background(), repo, policyEntry, entry, policyEntry, refName)
	assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
}

func TestVerifyEntry(t *testing.T) {
	// FIXME: currently this test is nearly identical to the one for VerifyRef.
	// This is because it's not trivial to create a bunch of test policy / RSL
	// states cleanly. We need something that is easy to maintain and add cases
	// to.
	repo, state := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1)
	entry := rsl.NewEntry(refName, commitIDs[0])
	entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
	entry.ID = entryID

	err := verifyEntry(context.Background(), repo, state, entry)
	assert.Nil(t, err)

	// FIXME: test for file policy passing for situations where a commit is seen
	// by the RSL before its signing key is rotated out. This commit should be
	// trusted for merges under the new policy because it predates the policy
	// change. This only applies to fast forwards, any other commits that make
	// the same semantic change will result in a new commit with a new
	// signature, unseen by the RSL.
}

func TestGetCommits(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	refName := "refs/heads/main"

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	// FIXME: this setup with RSL entries can be formalized using another
	// helper like createTestStateWithPolicy. The RSL could then also
	// incorporate policy changes and so on.
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5)
	firstEntry := rsl.NewEntry(refName, commitIDs[0])
	firstEntryID := common.CreateTestRSLEntryCommit(t, repo, firstEntry)
	firstEntry.ID = firstEntryID

	secondEntry := rsl.NewEntry(refName, commitIDs[4])
	secondEntryID := common.CreateTestRSLEntryCommit(t, repo, secondEntry)
	secondEntry.ID = secondEntryID

	expectedCommitIDs := []plumbing.Hash{commitIDs[1], commitIDs[2], commitIDs[3], commitIDs[4]}
	expectedCommits := make([]*object.Commit, 0, len(expectedCommitIDs))
	for _, commitID := range expectedCommitIDs {
		commit, err := repo.CommitObject(commitID)
		if err != nil {
			t.Fatal(err)
		}

		expectedCommits = append(expectedCommits, commit)
	}

	sort.Slice(expectedCommits, func(i, j int) bool {
		return expectedCommits[i].ID().String() < expectedCommits[j].ID().String()
	})

	commits, err := getCommits(repo, secondEntry)
	assert.Nil(t, err)
	assert.Equal(t, expectedCommits, commits)
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
