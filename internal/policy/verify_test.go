package policy

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/adityasaky/gittuf/internal/gitinterface"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
)

func TestVerifyRef(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	entry := rsl.NewEntry("refs/heads/main", plumbing.ZeroHash)
	createTestRSLEntryCommit(t, repo, entry)

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
	createTestRSLEntryCommit(t, repo, entry)

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
	entryID := createTestRSLEntryCommit(t, repo, entry)
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
	entryID := createTestRSLEntryCommit(t, repo, entry)
	entry.ID = entryID

	err := verifyEntry(context.Background(), repo, state, entry)
	assert.Nil(t, err)
}

func createTestRSLEntryCommit(t *testing.T, repo *git.Repository, entry *rsl.Entry) plumbing.Hash {
	t.Helper()

	// We do this manually because rsl.Commit() will not sign using our test key

	lines := []string{
		rsl.EntryHeader,
		"",
		fmt.Sprintf("%s: %s", rsl.RefKey, entry.RefName),
		fmt.Sprintf("%s: %s", rsl.CommitIDKey, entry.CommitID.String()),
	}

	commitMessage := strings.Join(lines, "\n")

	ref, err := repo.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}

	clock := clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
	testCommit := &object.Commit{
		Author: object.Signature{
			Name:  "Jane Doe",
			Email: "jane.doe@example.com",
			When:  clock.Now(),
		},
		Committer: object.Signature{
			Name:  "Jane Doe",
			Email: "jane.doe@example.com",
			When:  clock.Now(),
		},
		Message:      commitMessage,
		TreeHash:     plumbing.ZeroHash,
		ParentHashes: []plumbing.Hash{ref.Hash()},
	}

	testCommit = signTestCommit(t, repo, testCommit)

	if err := gitinterface.ApplyCommit(repo, testCommit, ref); err != nil {
		t.Fatal(err)
	}

	ref, err = repo.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}

	return ref.Hash()
}

func signTestCommit(t *testing.T, repo *git.Repository, commit *object.Commit) *object.Commit {
	t.Helper()

	commitEncoded := repo.Storer.NewEncodedObject()
	if err := commit.EncodeWithoutSignature(commitEncoded); err != nil {
		t.Fatal(err)
	}
	r, err := commitEncoded.Reader()
	if err != nil {
		t.Fatal(err)
	}

	signingKeyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-privkey.asc"))
	if err != nil {
		t.Fatal(err)
	}

	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(signingKeyBytes))
	if err != nil {
		t.Fatal(err)
	}

	sig := new(strings.Builder)
	if err := openpgp.ArmoredDetachSign(sig, keyring[0], r, nil); err != nil {
		t.Fatal(err)
	}
	commit.PGPSignature = sig.String()

	return commit
}
