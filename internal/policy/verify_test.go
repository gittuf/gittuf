// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
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
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	common.CreateTestRSLReferenceEntryCommit(t, repo, entry)

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
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	common.CreateTestRSLReferenceEntryCommit(t, repo, entry)

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

	policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
	if err != nil {
		t.Fatal(err)
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1)
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
	entry.ID = entryID

	err = VerifyRelativeForRef(context.Background(), repo, policyEntry, policyEntry, entry, refName)
	assert.Nil(t, err)

	err = VerifyRelativeForRef(context.Background(), repo, policyEntry, entry, policyEntry, refName)
	assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
}

func TestVerifyCommit(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"
	gpgKeyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}
	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3)
	entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
	entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
	entry.ID = entryID

	expectedStatus := make(map[string]string, len(commitIDs))
	commitIDStrings := make([]string, 0, len(commitIDs))
	for _, c := range commitIDs {
		commitIDStrings = append(commitIDStrings, c.String())
		expectedStatus[c.String()] = fmt.Sprintf(goodSignatureMessageFmt, gpgKey.KeyType, gpgKey.KeyID)
	}

	// Verify all commit signatures
	status := VerifyCommit(testCtx, repo, commitIDStrings...)
	assert.Equal(t, expectedStatus, status)

	if err := repo.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.ReferenceName(refName))); err != nil {
		t.Fatal(err)
	}

	// Verify signature for HEAD and branch
	expectedStatus = map[string]string{
		"HEAD":  fmt.Sprintf(goodSignatureMessageFmt, gpgKey.KeyType, gpgKey.KeyID),
		refName: fmt.Sprintf(goodSignatureMessageFmt, gpgKey.KeyType, gpgKey.KeyID),
	}
	status = VerifyCommit(testCtx, repo, "HEAD", refName)
	assert.Equal(t, expectedStatus, status)

	// Try a tag
	tagHash, err := gitinterface.Tag(repo, commitIDs[len(commitIDs)-1], "v1", "Test tag", false)
	if err != nil {
		t.Fatal(err)
	}

	expectedStatus = map[string]string{tagHash.String(): nonCommitMessage}
	status = VerifyCommit(testCtx, repo, tagHash.String())
	assert.Equal(t, expectedStatus, status)

	// Add a commit but don't record it in the RSL
	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1)

	expectedStatus = map[string]string{commitIDs[0].String(): unableToFindPolicyMessage}
	status = VerifyCommit(testCtx, repo, commitIDs[0].String())
	assert.Equal(t, expectedStatus, status)
}

func TestVerifyTag(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3)
	entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
	entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
	entry.ID = entryID

	tagName := "v1"
	tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1])

	expectedStatus := map[string]string{tagID.String(): unableToFindRSLEntryMessage}
	status := VerifyTag(context.Background(), repo, []string{tagID.String()})
	assert.Equal(t, expectedStatus, status)

	entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
	entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
	entry.ID = entryID

	// Use tag ID
	expectedStatus = map[string]string{tagID.String(): goodTagSignatureMessage}
	status = VerifyTag(context.Background(), repo, []string{tagID.String()})
	assert.Equal(t, expectedStatus, status)

	// Use tagName
	expectedStatus = map[string]string{tagName: goodTagSignatureMessage}
	status = VerifyTag(context.Background(), repo, []string{tagName})
	assert.Equal(t, expectedStatus, status)

	// Use refs path for tagName
	expectedStatus = map[string]string{string(plumbing.NewTagReferenceName(tagName)): goodTagSignatureMessage}
	status = VerifyTag(context.Background(), repo, []string{string(plumbing.NewTagReferenceName(tagName))})
	assert.Equal(t, expectedStatus, status)
}

func TestVerifyEntry(t *testing.T) {
	// FIXME: currently this test is nearly identical to the one for VerifyRef.
	// This is because it's not trivial to create a bunch of test policy / RSL
	// states cleanly. We need something that is easy to maintain and add cases
	// to.
	repo, state := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1)
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
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

func TestVerifyTagEntry(t *testing.T) {
	t.Run("no tag specific policy", func(t *testing.T) {
		repo, policy := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
		entry.ID = entryID

		tagName := "v1"
		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1])

		entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
		entry.ID = entryID

		err := verifyTagEntry(context.Background(), repo, policy, entry)
		assert.Nil(t, err)
	})

	t.Run("with tag specific policy", func(t *testing.T) {
		repo, policy := createTestRepository(t, createTestStateWithTagPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
		entry.ID = entryID

		tagName := "v1"
		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1])

		entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
		entry.ID = entryID

		err := verifyTagEntry(context.Background(), repo, policy, entry)
		assert.Nil(t, err)
	})

	t.Run("with tag specific policy, unauthorized", func(t *testing.T) {
		repo, policy := createTestRepository(t, createTestStateWithTagPolicyForUnauthorizedTest)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
		entry.ID = entryID

		tagName := "v1"
		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1])

		entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
		entry.ID = entryID

		err := verifyTagEntry(context.Background(), repo, policy, entry)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
	})
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
	firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
	firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry)
	firstEntry.ID = firstEntryID

	secondEntry := rsl.NewReferenceEntry(refName, commitIDs[4])
	secondEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, secondEntry)
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
	entries := []*rsl.ReferenceEntry{}
	for _, commitID := range commitIDs {
		entry := rsl.NewReferenceEntry(refName, commitID)
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry)
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

func TestStateVerifyNewState(t *testing.T) {
	t.Run("valid policy transition", func(t *testing.T) {
		currentPolicy := createTestStateWithOnlyRoot(t)
		newPolicy := createTestStateWithOnlyRoot(t)

		err := currentPolicy.VerifyNewState(context.Background(), newPolicy)
		assert.Nil(t, err)
	})

	t.Run("invalid policy transition", func(t *testing.T) {
		currentPolicy := createTestStateWithOnlyRoot(t)

		// Create invalid state
		signingKeyBytes, err := os.ReadFile(filepath.Join("test-data", "targets-1"))
		if err != nil {
			t.Fatal(err)
		}
		signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(signingKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		keyBytes, err := os.ReadFile(filepath.Join("test-data", "targets-1.pub"))
		if err != nil {
			t.Fatal(err)
		}
		key, err := tuf.LoadKeyFromBytes(keyBytes)
		if err != nil {
			t.Fatal(err)
		}

		rootMetadata := InitializeRootMetadata(key)

		rootEnv, err := dsse.CreateEnvelope(rootMetadata)
		if err != nil {
			t.Fatal(err)
		}
		rootEnv, err = dsse.SignEnvelope(context.Background(), rootEnv, signer)
		if err != nil {
			t.Fatal(err)
		}
		newPolicy := &State{
			RootPublicKeys:      []*tuf.Key{key},
			RootEnvelope:        rootEnv,
			DelegationEnvelopes: map[string]*sslibdsse.Envelope{},
		}

		err = currentPolicy.VerifyNewState(context.Background(), newPolicy)
		assert.ErrorContains(t, err, "do not match threshold")
	})
}
