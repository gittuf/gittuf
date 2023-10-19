// SPDX-License-Identifier: Apache-2.0

package common

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/jonboulle/clockwork"
)

const (
	testName  = "Jane Doe"
	testEmail = "jane.doe@example.com"
)

var (
	testGitConfig = &config.Config{
		User: struct {
			Name  string
			Email string
		}{
			Name:  testName,
			Email: testEmail,
		},
	}
	testClock = clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
)

// CreateTestRSLReferenceEntryCommit is a test helper used to create a
// **signed** reference entry using the GPG key stored in the repository. It is
// used to substitute for the default RSL entry creation and signing mechanism
// which relies on the user's Git config.
func CreateTestRSLReferenceEntryCommit(t *testing.T, repo *git.Repository, entry *rsl.ReferenceEntry) plumbing.Hash {
	t.Helper()

	// We do this manually because rsl.Commit() will not sign using our test key

	lines := []string{
		rsl.ReferenceEntryHeader,
		"",
		fmt.Sprintf("%s: %s", rsl.RefKey, entry.RefName),
		fmt.Sprintf("%s: %s", rsl.TargetIDKey, entry.TargetID.String()),
	}

	commitMessage := strings.Join(lines, "\n")

	ref, err := repo.Reference(plumbing.ReferenceName(rsl.Ref), true)
	if err != nil {
		t.Fatal(err)
	}

	testCommit := &object.Commit{
		Author: object.Signature{
			Name:  testName,
			Email: testEmail,
			When:  testClock.Now(),
		},
		Committer: object.Signature{
			Name:  testName,
			Email: testEmail,
			When:  testClock.Now(),
		},
		Message:      commitMessage,
		TreeHash:     gitinterface.EmptyTree(),
		ParentHashes: []plumbing.Hash{ref.Hash()},
	}

	testCommit = SignTestCommit(t, repo, testCommit)

	commitID, err := gitinterface.ApplyCommit(repo, testCommit, ref)
	if err != nil {
		t.Fatal(err)
	}

	return commitID
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

// SignTestTag signs the test tag using the test key stored in the repository.
// Note that the GPG key is loaded relative to the package containing the test.
func SignTestTag(t *testing.T, repo *git.Repository, tag *object.Tag) *object.Tag {
	t.Helper()

	tagEncoded := repo.Storer.NewEncodedObject()
	if err := tag.EncodeWithoutSignature(tagEncoded); err != nil {
		t.Fatal(err)
	}
	r, err := tagEncoded.Reader()
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
	tag.PGPSignature = sig.String()

	return tag
}

// AddNTestCommitsToSpecifiedRef is a test helper that adds test commits to the
// specified Git ref in the provided repository. Parameter `n` determines how
// many commits are added. Each commit is associated with a distinct tree. The
// first commit contains a tree with one object (an empty blob), the second with
// two objects (both empty blobs), and so on.
func AddNTestCommitsToSpecifiedRef(t *testing.T, repo *git.Repository, refName string, n int) []plumbing.Hash {
	t.Helper()

	emptyBlobHash, err := gitinterface.WriteBlob(repo, []byte{})
	if err != nil {
		t.Fatal(err)
	}

	// Create N trees with 1...N artifacts
	treeHashes := make([]plumbing.Hash, 0, n)
	for i := 1; i <= n; i++ {
		objects := make([]object.TreeEntry, 0, i)
		for j := 0; j < i; j++ {
			objects = append(objects, object.TreeEntry{Name: fmt.Sprintf("%d", j+1), Hash: emptyBlobHash})
		}

		treeHash, err := gitinterface.WriteTree(repo, objects)
		if err != nil {
			t.Fatal(err)
		}

		treeHashes = append(treeHashes, treeHash)
	}

	refNameTyped := plumbing.ReferenceName(refName)

	ref, err := repo.Reference(refNameTyped, true)
	if err != nil {
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			if err := repo.Storer.SetReference(plumbing.NewHashReference(refNameTyped, plumbing.ZeroHash)); err != nil {
				t.Fatal(err)
			}
			ref, err = repo.Reference(refNameTyped, true)
			if err != nil {
				t.Fatal(err)
			}
		} else {
			t.Fatal(err)
		}
	}

	commitIDs := []plumbing.Hash{}
	for i := 0; i < n; i++ {
		commit := gitinterface.CreateCommitObject(testGitConfig, treeHashes[i], ref.Hash(), "Test commit", testClock)
		commit = SignTestCommit(t, repo, commit)
		if _, err := gitinterface.ApplyCommit(repo, commit, ref); err != nil {
			t.Fatal(err)
		}

		// we need to re-set ref because it needs to be updated for the next
		// iteration
		ref, err = repo.Reference(refNameTyped, true)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs = append(commitIDs, ref.Hash())
	}

	return commitIDs
}

func CreateTestSignedTag(t *testing.T, repo *git.Repository, tagName string, target plumbing.Hash) plumbing.Hash {
	t.Helper()

	targetObj, err := repo.Object(plumbing.AnyObject, target)
	if err != nil {
		t.Fatal(err)
	}

	tagMessage := fmt.Sprintf("%s\n", tagName)
	tag := gitinterface.CreateTagObject(testGitConfig, targetObj, tagName, tagMessage, testClock)
	tag = SignTestTag(t, repo, tag)
	tagHash, err := gitinterface.ApplyTag(repo, tag)
	if err != nil {
		t.Fatal(err)
	}

	return tagHash
}
