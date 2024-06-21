// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/pem"
	"fmt"
	"strings"
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
func CreateTestRSLReferenceEntryCommit(t *testing.T, repo *gitinterface.Repository, entry *rsl.ReferenceEntry, signingKeyBytes []byte) gitinterface.Hash {
	t.Helper()

	// We do this manually because rsl.Commit() will not sign using our test key

	lines := []string{
		rsl.ReferenceEntryHeader,
		"",
		fmt.Sprintf("%s: %s", rsl.RefKey, entry.RefName),
		fmt.Sprintf("%s: %s", rsl.TargetIDKey, entry.TargetID.String()),
	}

	commitMessage := strings.Join(lines, "\n")

	treeBuilder := gitinterface.NewReplacementTreeBuilder(repo)
	emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitIDHash, err := repo.CommitUsingSpecificKey(emptyTreeHash, rsl.Ref, commitMessage, signingKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	return commitIDHash
}

// CreateTestRSLAnnotationEntryCommit is a test helper used to create a
// **signed** RSL annotation using the specified GPG key. It is used to
// substitute for the default RSL annotation creation and signing mechanism
// which relies on the user's Git config.
func CreateTestRSLAnnotationEntryCommit(t *testing.T, repo *gitinterface.Repository, annotation *rsl.AnnotationEntry, signingKeyBytes []byte) gitinterface.Hash {
	t.Helper()

	// We do this manually because rsl.Commit() will not sign using our test key

	lines := []string{
		rsl.AnnotationEntryHeader,
		"",
	}

	for _, entry := range annotation.RSLEntryIDs {
		lines = append(lines, fmt.Sprintf("%s: %s", rsl.EntryIDKey, entry.String()))
	}

	if annotation.Skip {
		lines = append(lines, fmt.Sprintf("%s: true", rsl.SkipKey))
	} else {
		lines = append(lines, fmt.Sprintf("%s: false", rsl.SkipKey))
	}

	if len(annotation.Message) != 0 {
		var message strings.Builder
		messageBlock := pem.Block{
			Type:  rsl.AnnotationMessageBlockType,
			Bytes: []byte(annotation.Message),
		}
		if err := pem.Encode(&message, &messageBlock); err != nil {
			t.Fatal(err)
		}
		lines = append(lines, strings.TrimSpace(message.String()))
	}

	commitMessage := strings.Join(lines, "\n")

	treeBuilder := gitinterface.NewReplacementTreeBuilder(repo)
	emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
	if err != nil {
		t.Fatal(err)
	}

	commitIDHash, err := repo.CommitUsingSpecificKey(emptyTreeHash, rsl.Ref, commitMessage, signingKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	return commitIDHash
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

	treeBuilder := gitinterface.NewReplacementTreeBuilder(repo)

	// Create N trees with 1...N artifacts
	treeHashes := make([]gitinterface.Hash, 0, n)
	for i := 1; i <= n; i++ {
		objects := map[string]gitinterface.Hash{}
		for j := 0; j < i; j++ {
			objects[fmt.Sprintf("%d", j+1)] = emptyBlobHash
		}

		treeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(objects)
		if err != nil {
			t.Fatal(err)
		}

		treeHashes = append(treeHashes, treeHash)
	}

	commitIDs := []gitinterface.Hash{}
	for i := 0; i < n; i++ {
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
