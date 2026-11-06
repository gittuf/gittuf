// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common //nolint:revive

import (
	"fmt"
	"testing"
	"time"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/go-git/go-git/v6"
	"github.com/go-git/go-git/v6/config"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/jonboulle/clockwork"
)

const (
	testName  = "Jane Doe"
	testEmail = "jane.doe@example.com"
)

var (
	TestGitConfig = &config.Config{
		User: struct {
			Name       string
			Email      string
			SigningKey string
		}{
			Name:  testName,
			Email: testEmail,
		},
	}
	TestClock = clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
)

// CreateTestRSLReferenceEntryCommit is a test helper used to create a
// **signed** reference entry using the specified GPG key. It is used to
// substitute for the default RSL entry creation and signing mechanism which
// relies on the user's Git config.
//
// Update: This helper just wraps around CommitUsingSpecificKey in the rsl
// package. We can probably get rid of it, but it's a pretty big delta.
func CreateTestRSLReferenceEntryCommit(t *testing.T, repo gitstore.Storer, entry *rsl.ReferenceEntry, signingKeyBytes []byte) githash.Hash {
	t.Helper()

	if err := entry.CommitUsingSpecificKey(repo, signingKeyBytes); err != nil {
		t.Fatal(err)
	}

	entryID, err := repo.GetReference(rsl.Ref)
	if err != nil {
		t.Fatal(err)
	}

	return entryID
}

// CreateTestRSLAnnotationEntryCommit is a test helper used to create a
// **signed** RSL annotation using the specified GPG key. It is used to
// substitute for the default RSL annotation creation and signing mechanism
// which relies on the user's Git config.
//
// Update: This helper just wraps around CommitUsingSpecificKey in the rsl
// package. We can probably get rid of it, but it's a pretty big delta.
func CreateTestRSLAnnotationEntryCommit(t *testing.T, repo gitstore.Storer, annotation *rsl.AnnotationEntry, signingKeyBytes []byte) githash.Hash {
	t.Helper()

	if err := annotation.CommitUsingSpecificKey(repo, signingKeyBytes); err != nil {
		t.Fatal(err)
	}

	entryID, err := repo.GetReference(rsl.Ref)
	if err != nil {
		t.Fatal(err)
	}

	return entryID
}

// AddNTestCommitsToSpecifiedRef is a test helper that adds test commits to the
// specified Git ref in the provided repository. Parameter `n` determines how
// many commits are added. Each commit is associated with a distinct tree. The
// first commit contains a tree with one object (an empty blob), the second with
// two objects (both empty blobs), and so on. Each commit is signed using the
// specified key.
func AddNTestCommitsToSpecifiedRef(t *testing.T, repo *gitinterface.Repository, refName string, n int, signingKeyBytes []byte) []githash.Hash {
	t.Helper()

	emptyBlobHash, err := repo.WriteBlob(nil)
	if err != nil {
		t.Fatal(err)
	}

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	// Create N trees with 1...N artifacts
	treeHashes := make([]githash.Hash, 0, n)
	for i := range n {
		objects := []gitinterface.TreeEntry{}
		for j := range i + 1 {
			objects = append(objects, gitinterface.NewEntryBlob(fmt.Sprintf("%d", j+1), emptyBlobHash))
		}

		treeHash, err := treeBuilder.WriteTreeFromEntries(objects)
		if err != nil {
			t.Fatal(err)
		}

		treeHashes = append(treeHashes, treeHash)
	}

	commitIDs := []githash.Hash{}
	for i := range n {
		commitID, err := repo.CommitUsingSpecificKey(treeHashes[i], refName, "Test commit\n", signingKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs = append(commitIDs, commitID)
	}

	return commitIDs
}

// CreateTestSignedTag creates a signed tag in the repository pointing to the
// target object. The tag is signed using the specified key.
func CreateTestSignedTag(t *testing.T, repo *gitinterface.Repository, tagName string, target githash.Hash, signingKeyBytes []byte) githash.Hash {
	t.Helper()

	tagMessage := fmt.Sprintf("%s\n", tagName)
	tagID, err := repo.TagUsingSpecificKey(target, tagName, tagMessage, signingKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	return tagID
}

// CreateTestTagWithCompatSignature creates a tag whose `gpgsig-sha256` header
// is signed using compatSigningKeyBytes. The appended signature is made using
// signingKeyBytes, or left out when it is nil.
func CreateTestTagWithCompatSignature(t *testing.T, repo *gitinterface.Repository, tagName string, target githash.Hash, signingKeyBytes, compatSigningKeyBytes []byte) githash.Hash {
	t.Helper()

	goGitRepo, err := repo.GetGoGitRepository()
	if err != nil {
		t.Fatal(err)
	}

	// The fixed clock makes every tag created here share a payload.
	signedTag := func(keyBytes []byte) *object.Tag {
		tagID := CreateTestSignedTag(t, repo, tagName, target, keyBytes)
		tag, err := goGitRepo.TagObject(plumbing.NewHash(tagID.String()))
		if err != nil {
			t.Fatal(err)
		}
		return tag
	}

	compatTag := signedTag(compatSigningKeyBytes)
	tag := &object.Tag{
		Name:            compatTag.Name,
		Tagger:          compatTag.Tagger,
		Message:         compatTag.Message,
		TargetType:      compatTag.TargetType,
		Target:          compatTag.Target,
		SignatureSHA256: compatTag.Signature,
	}
	if signingKeyBytes != nil {
		tag.Signature = signedTag(signingKeyBytes).Signature
	}

	obj := goGitRepo.Storer.NewEncodedObject()
	if err := tag.Encode(obj); err != nil {
		t.Fatal(err)
	}
	objID, err := goGitRepo.Storer.SetEncodedObject(obj)
	if err != nil {
		t.Fatal(err)
	}

	tagID, err := gitinterface.NewHash(objID.String())
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.SetReference(gitinterface.TagReferenceName(tagName), tagID); err != nil {
		t.Fatal(err)
	}

	return tagID
}

// CreateTestUnsignedTag creates an unsigned annotated tag in the repository
// pointing to the target object.
func CreateTestUnsignedTag(t *testing.T, repo *gitinterface.Repository, tagName string, target githash.Hash) githash.Hash {
	t.Helper()

	goGitRepo, err := repo.GetGoGitRepository()
	if err != nil {
		t.Fatal(err)
	}

	tagRef, err := goGitRepo.CreateTag(tagName, plumbing.NewHash(target.String()), &git.CreateTagOptions{
		Tagger:  &object.Signature{Name: testName, Email: testEmail, When: TestClock.Now()},
		Message: fmt.Sprintf("%s\n", tagName),
	})
	if err != nil {
		t.Fatal(err)
	}

	tagID, err := gitinterface.NewHash(tagRef.Hash().String())
	if err != nil {
		t.Fatal(err)
	}

	return tagID
}
