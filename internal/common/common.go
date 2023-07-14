package common

import (
	"bytes"
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
)

// CreateTestRSLEntryCommit is a test helper used to create a **signed** RSL
// entry using the GPG key stored in the repository. It is used to substitute
// for the default RSL entry creation and signing mechanism which relies on the
// user's Git config.
func CreateTestRSLEntryCommit(t *testing.T, repo *git.Repository, entry *rsl.Entry) plumbing.Hash {
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

	testCommit = SignTestCommit(t, repo, testCommit)

	if err := gitinterface.ApplyCommit(repo, testCommit, ref); err != nil {
		t.Fatal(err)
	}

	ref, err = repo.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}

	return ref.Hash()
}

// SignTestCommit signs the test commit using the test key stored in the
// repository. Note that the GPG key is loaded relative to the package
// containing the test.
func SignTestCommit(t *testing.T, repo *git.Repository, commit *object.Commit) *object.Commit {
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
