package policy

import (
	"context"
	"testing"
	"time"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/gitinterface"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/jonboulle/clockwork"
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

	refName := "refs/heads/main"
	commitIDs := addTestCommits(t, repo, refName)

	for _, commitID := range commitIDs {
		entry := rsl.NewEntry("refs/heads/main", commitID)
		entryID := common.CreateTestRSLEntryCommit(t, repo, entry)
		entry.ID = entryID

		err := verifyEntry(context.Background(), repo, state, entry)
		assert.Nil(t, err)
	}
}

func addTestCommits(t *testing.T, repo *git.Repository, refName string) []plumbing.Hash {
	t.Helper()
	commitIDs := []plumbing.Hash{}

	refNameTyped := plumbing.ReferenceName(refName)
	repo.Storer.SetReference(plumbing.NewHashReference(refNameTyped, plumbing.ZeroHash))
	ref, err := repo.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}

	gitConfig := &config.Config{
		Raw: &format.Config{
			Sections: format.Sections{
				&format.Section{
					Name: "user",
					Options: format.Options{
						&format.Option{
							Key:   "name",
							Value: "Jane Doe",
						},
						&format.Option{
							Key:   "email",
							Value: "jane.doe@example.com",
						},
					},
				},
			},
		},
	}

	clock := clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
	commit := gitinterface.CreateCommitObject(gitConfig, gitinterface.EmptyTree(), plumbing.ZeroHash, "Test commit", clock)
	gitinterface.ApplyCommit(repo, commit, ref)

	ref, err = repo.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}

	commitIDs = append(commitIDs, ref.Hash())

	commit = gitinterface.CreateCommitObject(gitConfig, gitinterface.EmptyTree(), ref.Hash(), "Test commit", clock)
	gitinterface.ApplyCommit(repo, commit, ref)

	ref, err = repo.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}

	commitIDs = append(commitIDs, ref.Hash())

	return commitIDs
}
