// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common //nolint:revive

import (
	"fmt"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/config"
	"github.com/jonboulle/clockwork"
)

const (
	testName  = "Jane Doe"
	testEmail = "jane.doe@example.com"
)

var (
	TestGitConfig = &config.Config{
		User: struct {
			Name  string
			Email string
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
func CreateTestRSLReferenceEntryCommit(t *testing.T, repo *gitinterface.Repository, entry *rsl.ReferenceEntry, signingKeyBytes []byte) gitinterface.Hash {
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
func CreateTestRSLAnnotationEntryCommit(t *testing.T, repo *gitinterface.Repository, annotation *rsl.AnnotationEntry, signingKeyBytes []byte) gitinterface.Hash {
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
func AddNTestCommitsToSpecifiedRef(t *testing.T, repo *gitinterface.Repository, refName string, n int, signingKeyBytes []byte) []gitinterface.Hash {
	t.Helper()

	emptyBlobHash, err := repo.WriteBlob(nil)
	if err != nil {
		t.Fatal(err)
	}

	treeBuilder := gitinterface.NewTreeBuilder(repo)

	// Create N trees with 1...N artifacts
	treeHashes := make([]gitinterface.Hash, 0, n)
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

	commitIDs := []gitinterface.Hash{}
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
func CreateTestSignedTag(t *testing.T, repo *gitinterface.Repository, tagName string, target gitinterface.Hash, signingKeyBytes []byte) gitinterface.Hash {
	t.Helper()

	tagMessage := fmt.Sprintf("%s\n", tagName)
	tagID, err := repo.TagUsingSpecificKey(target, tagName, tagMessage, signingKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	return tagID
}
