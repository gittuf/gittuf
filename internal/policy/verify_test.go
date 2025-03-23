// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"os"
	"path/filepath"
	"sort"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// FIXME: the verification tests do not check for expected failures. More
// broadly, we need to rework the test setup here starting with
// createTestRepository and the state creation helpers.

func TestVerifyRef(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)

	verifier := NewPolicyVerifier(repo)

	currentTip, err := verifier.VerifyRef(testCtx, refName)
	assert.Nil(t, err)
	assert.Equal(t, commitIDs[0], currentTip)
}

func TestVerifyRefFull(t *testing.T) {
	// FIXME: currently this test is identical to the one for VerifyRef.
	// This is because it's not trivial to create a bunch of test policy / RSL
	// states cleanly. We need something that is easy to maintain and add cases
	// to.
	repo, _ := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)

	verifier := NewPolicyVerifier(repo)

	currentTip, err := verifier.VerifyRefFull(testCtx, refName)
	assert.Nil(t, err)
	assert.Equal(t, commitIDs[0], currentTip)
}

func TestVerifyRefFromEntry(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"

	// Policy violation
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgUnauthorizedKeyBytes)
	entry := rsl.NewReferenceEntry(refName, commitIDs[2])
	common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)

	// Not policy violation by itself
	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
	entry = rsl.NewReferenceEntry(refName, commitIDs[2])
	entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)

	// Not policy violation by itself
	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
	entry = rsl.NewReferenceEntry(refName, commitIDs[1])
	common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)

	verifier := NewPolicyVerifier(repo)

	// Verification passes because it's from a non-violating state only
	currentTip, err := verifier.VerifyRefFromEntry(testCtx, refName, entryID)
	assert.Nil(t, err)
	assert.Equal(t, commitIDs[1], currentTip)
}

func TestVerifyRelativeForRefUsingPersons(t *testing.T) {
	t.Setenv(tufv02.AllowV02MetadataKey, "1")
	t.Setenv(dev.DevModeKey, "1")

	t.Run("no recovery", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		err = verifier.VerifyRelativeForRef(testCtx, entry, firstEntry, refName)
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
	})

	t.Run("no recovery, first entry is the very first entry", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		firstEntry, _, err := rsl.GetFirstEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		err = verifier.VerifyRelativeForRef(testCtx, entry, firstEntry, refName)
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
	})

	t.Run("no recovery, first entry is the very first entry but policy is not applied", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		currentRSLTip, err := repo.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}
		currentRSLTipParentIDs, err := repo.GetCommitParentIDs(currentRSLTip)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.SetReference(rsl.Ref, currentRSLTipParentIDs[0]); err != nil {
			// Set to parent -> this is policy staging
			t.Fatal(err)
		}

		refName := "refs/heads/main"

		firstEntry, _, err := rsl.GetFirstEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrPolicyNotFound)
	})

	t.Run("with recovery, commit-same, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, commit-same, recovered by unauthorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgUnauthorizedKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, tree-same, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit's tree
		validTreeID, err := repo.GetCommitTreeID(validCommitID)
		if err != nil {
			t.Fatal(err)
		}

		newCommitID, err := repo.CommitUsingSpecificKey(validTreeID, refName, "Revert invalid commit\n", gpgKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, newCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, tree-same, recovered by unauthorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit's tree
		validTreeID, err := repo.GetCommitTreeID(validCommitID)
		if err != nil {
			t.Fatal(err)
		}

		newCommitID, err := repo.CommitUsingSpecificKey(validTreeID, refName, "Revert invalid commit\n", gpgUnauthorizedKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgUnauthorizedKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, newCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, commit-same, multiple invalid entries, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		invalidEntryIDs := []gitinterface.Hash{entryID}

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's still in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		invalidEntryIDs = append(invalidEntryIDs, entryID)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entries
		annotation := rsl.NewAnnotationEntry(invalidEntryIDs, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, commit-same, unskipped invalid entries, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		invalidEntryIDs := []gitinterface.Hash{entryID}

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's still in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for only one invalid entry
		annotation := rsl.NewAnnotationEntry(invalidEntryIDs, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// An invalid entry is not marked as skipped
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrInvalidEntryNotSkipped)
	})

	t.Run("with recovery, commit-same, recovered by authorized user, last good state is due to recovery", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		// Send it into invalid state again
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation = rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID = common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, error because recovery goes back too far, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		invalidLastGoodCommitID := commitIDs[len(commitIDs)-1]

		// Add more commits, change the number of commits to have different trees
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 4, gpgKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the invalid last good commit
		if err := repo.SetReference(refName, invalidLastGoodCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to invalid last good commit
		entry = rsl.NewReferenceEntry(refName, invalidLastGoodCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("with recovery but recovered entry is also skipped, tree-same, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit's tree
		validTreeID, err := repo.GetCommitTreeID(validCommitID)
		if err != nil {
			t.Fatal(err)
		}

		newCommitID, err := repo.CommitUsingSpecificKey(validTreeID, refName, "Revert invalid commit\n", gpgKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, newCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		// Skip the recovery entry as well
		annotation = rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID = common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("with annotation but no fix entry", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicyUsingPersons)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID

		// No fix entry, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})
}

func TestVerifyMergeable(t *testing.T) {
	refName := "refs/heads/main"
	featureRefName := "refs/heads/feature"

	t.Setenv(dev.DevModeKey, "1")
	t.Setenv(tufv02.AllowV02MetadataKey, "1")

	t.Run("base commit zero, mergeable using GitHub approval, RSL entry signature required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(featureRefName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Set up approval attestation with "john.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"john.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeable(testCtx, refName, featureRefName)
		assert.Nil(t, err)
		assert.True(t, rslSignatureRequired)
	})

	t.Run("base commit zero, mergeable using mixed approvals, RSL entry signature required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrustForMixedAttestations)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(featureRefName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Set up approval attestation with "jill.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"jill.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}

		// Set up reference authorization from "john.doe"
		refAuthorization, err := attestations.NewReferenceAuthorizationForCommit(refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		// This signer is for the SSH keys associated with john.doe
		signer = setupSSHKeysForSigning(t, targets2KeyBytes, targets2PubKeyBytes)

		env, err = dsse.CreateEnvelope(refAuthorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval and reference authorization", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeable(testCtx, refName, featureRefName)
		assert.Nil(t, err)
		assert.True(t, rslSignatureRequired)
	})

	t.Run("base commit zero, mergeable using GitHub approval, RSL entry signature not required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(featureRefName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Add approval with "jane.doe" and "john.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"jane.doe", "john.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeable(testCtx, refName, featureRefName)
		assert.Nil(t, err)
		assert.False(t, rslSignatureRequired)
	})

	t.Run("base commit zero, not mergeable", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(featureRefName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Add approval with "alice" and "bob"
		// These are untrusted identities
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"alice", "bob"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeable(testCtx, refName, featureRefName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
		assert.False(t, rslSignatureRequired)
	})

	t.Run("base commit not zero, mergeable using GitHub approval, RSL entry signature required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		baseCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
		baseEntry := rsl.NewReferenceEntry(refName, baseCommitIDs[1])
		baseEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, baseEntry, gpgKeyBytes)
		baseEntry.ID = baseEntryID

		if err := repo.SetReference("HEAD", baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}
		repo.RestoreWorktree(t)

		// Set feature to the same commit as main
		if err := repo.SetReference(featureRefName, baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 2, gpgKeyBytes)
		featureEntry := rsl.NewReferenceEntry(featureRefName, featureCommitIDs[1]) // latest commit
		featureEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, featureEntry, gpgKeyBytes)
		featureEntry.ID = featureEntryID

		commitTreeID, err := repo.GetCommitTreeID(featureCommitIDs[1]) // latest commit
		if err != nil {
			t.Fatal(err)
		}

		// Set up approval attestation with "john.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, baseCommitIDs[1].String(), commitTreeID.String(), []string{"john.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, baseCommitIDs[1].String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeable(testCtx, refName, featureRefName)
		assert.Nil(t, err)
		assert.True(t, rslSignatureRequired)
	})

	t.Run("base commit not zero, mergeable using mixed approvals, RSL entry signature required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrustForMixedAttestations)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		baseCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
		baseEntry := rsl.NewReferenceEntry(refName, baseCommitIDs[1])
		baseEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, baseEntry, gpgKeyBytes)
		baseEntry.ID = baseEntryID

		if err := repo.SetReference("HEAD", baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}
		repo.RestoreWorktree(t)

		// Set feature to the same commit as main
		if err := repo.SetReference(featureRefName, baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 2, gpgKeyBytes)
		featureEntry := rsl.NewReferenceEntry(featureRefName, featureCommitIDs[1]) // latest commit
		featureEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, featureEntry, gpgKeyBytes)
		featureEntry.ID = featureEntryID

		commitTreeID, err := repo.GetCommitTreeID(featureCommitIDs[1]) // latest commit
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Set up approval attestation with "jill.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, baseCommitIDs[1].String(), commitTreeID.String(), []string{"jill.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, baseCommitIDs[1].String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}

		// Set up reference authorization from "john.doe"
		refAuthorization, err := attestations.NewReferenceAuthorizationForCommit(refName, baseCommitIDs[1].String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		// This is the key associated with john.doe
		signer = setupSSHKeysForSigning(t, targets2KeyBytes, targets2PubKeyBytes)

		env, err = dsse.CreateEnvelope(refAuthorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, baseCommitIDs[1].String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval and reference authorization", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeable(testCtx, refName, featureRefName)
		assert.Nil(t, err)
		assert.True(t, rslSignatureRequired)
	})

	t.Run("base commit not zero, mergeable using GitHub approval, RSL entry signature not required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		baseCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
		baseEntry := rsl.NewReferenceEntry(refName, baseCommitIDs[1])
		baseEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, baseEntry, gpgKeyBytes)
		baseEntry.ID = baseEntryID

		if err := repo.SetReference("HEAD", baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}
		repo.RestoreWorktree(t)

		// Set feature to the same commit as main
		if err := repo.SetReference(featureRefName, baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 2, gpgKeyBytes)
		featureEntry := rsl.NewReferenceEntry(featureRefName, featureCommitIDs[1]) // latest commit
		featureEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, featureEntry, gpgKeyBytes)
		featureEntry.ID = featureEntryID

		commitTreeID, err := repo.GetCommitTreeID(featureCommitIDs[1]) // latest commit
		if err != nil {
			t.Fatal(err)
		}

		// Add approval with "jane.doe" and "john.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, baseCommitIDs[1].String(), commitTreeID.String(), []string{"john.doe", "jane.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, baseCommitIDs[1].String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeable(testCtx, refName, featureRefName)
		assert.Nil(t, err)
		assert.False(t, rslSignatureRequired)
	})

	t.Run("base commit not zero, not mergeable", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		baseCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
		baseEntry := rsl.NewReferenceEntry(refName, baseCommitIDs[1])
		baseEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, baseEntry, gpgKeyBytes)
		baseEntry.ID = baseEntryID

		if err := repo.SetReference("HEAD", baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}
		repo.RestoreWorktree(t)

		// Set feature to the same commit as main
		if err := repo.SetReference(featureRefName, baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 2, gpgKeyBytes)
		featureEntry := rsl.NewReferenceEntry(featureRefName, featureCommitIDs[1]) // latest commit
		featureEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, featureEntry, gpgKeyBytes)
		featureEntry.ID = featureEntryID

		commitTreeID, err := repo.GetCommitTreeID(featureCommitIDs[1]) // latest commit
		if err != nil {
			t.Fatal(err)
		}

		// Add approval with "alice" and "bob"
		// These are untrusted approvals
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, baseCommitIDs[1].String(), commitTreeID.String(), []string{"alice", "bob"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, baseCommitIDs[1].String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeable(testCtx, refName, featureRefName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
		assert.False(t, rslSignatureRequired)
	})

	t.Run("unprotected base branch", func(t *testing.T) {
		refName := "refs/heads/unprotected" // overriding refName

		repo, _ := createTestRepository(t, createTestStateWithPolicy)

		baseCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
		baseEntry := rsl.NewReferenceEntry(refName, baseCommitIDs[1])
		baseEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, baseEntry, gpgKeyBytes)
		baseEntry.ID = baseEntryID

		if err := repo.SetReference("HEAD", baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		// Set feature to the same commit as base
		if err := repo.SetReference(featureRefName, baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 2, gpgKeyBytes)
		featureEntry := rsl.NewReferenceEntry(featureRefName, featureCommitIDs[1]) // latest commit
		featureEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, featureEntry, gpgKeyBytes)
		featureEntry.ID = featureEntryID

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeable(testCtx, refName, featureRefName)
		assert.Nil(t, err)
		assert.False(t, rslSignatureRequired)
	})
}

func TestVerifyMergeableForCommit(t *testing.T) {
	refName := "refs/heads/main"
	featureRefName := "refs/heads/feature"

	t.Run("base commit zero, mergeable using GitHub approval, RSL entry signature required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 1, gpgKeyBytes)
		featureID := commitIDs[0]

		commitTreeID, err := repo.GetCommitTreeID(featureID)
		if err != nil {
			t.Fatal(err)
		}

		// Set up approval attestation with "john.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"john.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeableForCommit(testCtx, refName, featureID)
		assert.Nil(t, err)
		assert.True(t, rslSignatureRequired)
	})

	t.Run("base commit zero, mergeable using mixed approvals, RSL entry signature required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrustForMixedAttestations)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 1, gpgKeyBytes)
		featureID := commitIDs[0]

		commitTreeID, err := repo.GetCommitTreeID(featureID)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Set up approval attestation with "jill.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"jill.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}

		// Set up reference authorization from "john.doe"
		refAuthorization, err := attestations.NewReferenceAuthorizationForCommit(refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		// This signer is for the SSH keys associated with john.doe
		signer = setupSSHKeysForSigning(t, targets2KeyBytes, targets2PubKeyBytes)

		env, err = dsse.CreateEnvelope(refAuthorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval and reference authorization", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeableForCommit(testCtx, refName, featureID)
		assert.Nil(t, err)
		assert.True(t, rslSignatureRequired)
	})

	t.Run("base commit zero, mergeable using GitHub approval, RSL entry signature not required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 1, gpgKeyBytes)
		featureID := commitIDs[0]

		commitTreeID, err := repo.GetCommitTreeID(featureID)
		if err != nil {
			t.Fatal(err)
		}

		// Add approval with "jane.doe" and "john.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"jane.doe", "john.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeableForCommit(testCtx, refName, featureID)
		assert.Nil(t, err)
		assert.False(t, rslSignatureRequired)
	})

	t.Run("base commit zero, not mergeable", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 1, gpgKeyBytes)
		featureID := commitIDs[0]

		commitTreeID, err := repo.GetCommitTreeID(featureID)
		if err != nil {
			t.Fatal(err)
		}

		// Add approval with "alice" and "bob"
		// These are untrusted identities
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"alice", "bob"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeableForCommit(testCtx, refName, featureID)
		assert.ErrorIs(t, err, ErrVerificationFailed)
		assert.False(t, rslSignatureRequired)
	})

	t.Run("base commit not zero, mergeable using GitHub approval, RSL entry signature required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		baseCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
		baseEntry := rsl.NewReferenceEntry(refName, baseCommitIDs[1])
		baseEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, baseEntry, gpgKeyBytes)
		baseEntry.ID = baseEntryID

		if err := repo.SetReference("HEAD", baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}
		repo.RestoreWorktree(t)

		// Set feature to the same commit as main
		if err := repo.SetReference(featureRefName, baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 2, gpgKeyBytes)
		featureID := featureCommitIDs[1]

		commitTreeID, err := repo.GetCommitTreeID(featureID) // latest commit
		if err != nil {
			t.Fatal(err)
		}

		// Set up approval attestation with "john.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, baseCommitIDs[1].String(), commitTreeID.String(), []string{"john.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, baseCommitIDs[1].String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeableForCommit(testCtx, refName, featureID)
		assert.Nil(t, err)
		assert.True(t, rslSignatureRequired)
	})

	t.Run("base commit not zero, mergeable using mixed approvals, RSL entry signature required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrustForMixedAttestations)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		baseCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
		baseEntry := rsl.NewReferenceEntry(refName, baseCommitIDs[1])
		baseEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, baseEntry, gpgKeyBytes)
		baseEntry.ID = baseEntryID

		if err := repo.SetReference("HEAD", baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}
		repo.RestoreWorktree(t)

		// Set feature to the same commit as main
		if err := repo.SetReference(featureRefName, baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 2, gpgKeyBytes)
		featureID := featureCommitIDs[1]

		commitTreeID, err := repo.GetCommitTreeID(featureID) // latest commit
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Set up approval attestation with "jill.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, baseCommitIDs[1].String(), commitTreeID.String(), []string{"jill.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, baseCommitIDs[1].String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}

		// Set up reference authorization from "john.doe"
		refAuthorization, err := attestations.NewReferenceAuthorizationForCommit(refName, baseCommitIDs[1].String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		// This is the key associated with john.doe
		signer = setupSSHKeysForSigning(t, targets2KeyBytes, targets2PubKeyBytes)

		env, err = dsse.CreateEnvelope(refAuthorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, baseCommitIDs[1].String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval and reference authorization", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeableForCommit(testCtx, refName, featureID)
		assert.Nil(t, err)
		assert.True(t, rslSignatureRequired)
	})

	t.Run("base commit not zero, mergeable using GitHub approval, RSL entry signature not required", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		baseCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
		baseEntry := rsl.NewReferenceEntry(refName, baseCommitIDs[1])
		baseEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, baseEntry, gpgKeyBytes)
		baseEntry.ID = baseEntryID

		if err := repo.SetReference("HEAD", baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}
		repo.RestoreWorktree(t)

		// Set feature to the same commit as main
		if err := repo.SetReference(featureRefName, baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 2, gpgKeyBytes)
		featureID := featureCommitIDs[1]

		commitTreeID, err := repo.GetCommitTreeID(featureID) // latest commit
		if err != nil {
			t.Fatal(err)
		}

		// Add approval with "jane.doe" and "john.doe"
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, baseCommitIDs[1].String(), commitTreeID.String(), []string{"john.doe", "jane.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, baseCommitIDs[1].String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeableForCommit(testCtx, refName, featureID)
		assert.Nil(t, err)
		assert.False(t, rslSignatureRequired)
	})

	t.Run("base commit not zero, not mergeable", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		baseCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
		baseEntry := rsl.NewReferenceEntry(refName, baseCommitIDs[1])
		baseEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, baseEntry, gpgKeyBytes)
		baseEntry.ID = baseEntryID

		if err := repo.SetReference("HEAD", baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}
		repo.RestoreWorktree(t)

		// Set feature to the same commit as main
		if err := repo.SetReference(featureRefName, baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 2, gpgKeyBytes)
		featureID := featureCommitIDs[1]

		commitTreeID, err := repo.GetCommitTreeID(featureID) // latest commit
		if err != nil {
			t.Fatal(err)
		}

		// Add approval with "alice" and "bob"
		// These are untrusted approvals
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, baseCommitIDs[1].String(), commitTreeID.String(), []string{"alice", "bob"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, baseCommitIDs[1].String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeableForCommit(testCtx, refName, featureID)
		assert.ErrorIs(t, err, ErrVerificationFailed)
		assert.False(t, rslSignatureRequired)
	})

	t.Run("unprotected base branch", func(t *testing.T) {
		refName := "refs/heads/unprotected" // overriding refName

		repo, _ := createTestRepository(t, createTestStateWithPolicy)

		// We need to change the directory for this test because we `checkout`
		// for older Git versions, modifying the worktree. This chdir ensures
		// that the temporary directory is used as the worktree.
		pwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(filepath.Join(repo.GetGitDir(), "..")); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(pwd) //nolint:errcheck

		baseCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
		baseEntry := rsl.NewReferenceEntry(refName, baseCommitIDs[1])
		baseEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, baseEntry, gpgKeyBytes)
		baseEntry.ID = baseEntryID

		if err := repo.SetReference("HEAD", baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		// Set feature to the same commit as base
		if err := repo.SetReference(featureRefName, baseCommitIDs[1]); err != nil {
			t.Fatal(err)
		}

		featureCommitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, featureRefName, 2, gpgKeyBytes)
		featureID := featureCommitIDs[1]
		repo.RestoreWorktree(t)

		verifier := NewPolicyVerifier(repo)
		rslSignatureRequired, err := verifier.VerifyMergeableForCommit(testCtx, refName, featureID)
		assert.Nil(t, err)
		assert.False(t, rslSignatureRequired)
	})
}

func TestVerifyRelativeForRef(t *testing.T) {
	t.Run("no recovery", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		err = verifier.VerifyRelativeForRef(testCtx, entry, firstEntry, refName)
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
	})

	t.Run("no recovery, first entry is the very first entry", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		firstEntry, _, err := rsl.GetFirstEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		err = verifier.VerifyRelativeForRef(testCtx, entry, firstEntry, refName)
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
	})

	t.Run("no recovery, first entry is the very first entry but policy is not applied", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		currentRSLTip, err := repo.GetReference(rsl.Ref)
		if err != nil {
			t.Fatal(err)
		}
		currentRSLTipParentIDs, err := repo.GetCommitParentIDs(currentRSLTip)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.SetReference(rsl.Ref, currentRSLTipParentIDs[0]); err != nil {
			// Set to parent -> this is policy staging
			t.Fatal(err)
		}

		refName := "refs/heads/main"

		firstEntry, _, err := rsl.GetFirstEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrPolicyNotFound)
	})

	t.Run("with recovery, commit-same, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, commit-same, recovered by unauthorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgUnauthorizedKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, tree-same, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit's tree
		validTreeID, err := repo.GetCommitTreeID(validCommitID)
		if err != nil {
			t.Fatal(err)
		}

		newCommitID, err := repo.CommitUsingSpecificKey(validTreeID, refName, "Revert invalid commit\n", gpgKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, newCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, tree-same, recovered by unauthorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit's tree
		validTreeID, err := repo.GetCommitTreeID(validCommitID)
		if err != nil {
			t.Fatal(err)
		}

		newCommitID, err := repo.CommitUsingSpecificKey(validTreeID, refName, "Revert invalid commit\n", gpgUnauthorizedKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgUnauthorizedKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, newCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, commit-same, multiple invalid entries, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		invalidEntryIDs := []gitinterface.Hash{entryID}

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's still in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		invalidEntryIDs = append(invalidEntryIDs, entryID)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entries
		annotation := rsl.NewAnnotationEntry(invalidEntryIDs, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, commit-same, unskipped invalid entries, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		invalidEntryIDs := []gitinterface.Hash{entryID}

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's still in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for only one invalid entry
		annotation := rsl.NewAnnotationEntry(invalidEntryIDs, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// An invalid entry is not marked as skipped
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrInvalidEntryNotSkipped)
	})

	t.Run("with recovery, commit-same, recovered by authorized user, last good state is due to recovery", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		// Send it into invalid state again
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit
		if err := repo.SetReference(refName, validCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation = rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID = common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, error because recovery goes back too far, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		invalidLastGoodCommitID := commitIDs[len(commitIDs)-1]

		// Add more commits, change the number of commits to have different trees
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 4, gpgKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the invalid last good commit
		if err := repo.SetReference(refName, invalidLastGoodCommitID); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to invalid last good commit
		entry = rsl.NewReferenceEntry(refName, invalidLastGoodCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("with recovery but recovered entry is also skipped, tree-same, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Fix using the known-good commit's tree
		validTreeID, err := repo.GetCommitTreeID(validCommitID)
		if err != nil {
			t.Fatal(err)
		}

		newCommitID, err := repo.CommitUsingSpecificKey(validTreeID, refName, "Revert invalid commit\n", gpgKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, newCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		// Skip the recovery entry as well
		annotation = rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID = common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("with annotation but no fix entry", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
		firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
		firstEntry.ID = firstEntryID

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		verifier := NewPolicyVerifier(repo)
		err := verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.Nil(t, err)

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID

		// No fix entry, error out
		verifier = NewPolicyVerifier(repo)
		err = verifier.VerifyRelativeForRef(testCtx, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})
}

func TestVerifyEntry(t *testing.T) {
	refName := "refs/heads/main"

	t.Run("successful verification", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithPolicy)

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err := verifyEntry(testCtx, repo, state, nil, entry)
		assert.Nil(t, err)
	})

	t.Run("successful verification using persons", func(t *testing.T) {
		t.Setenv(tufv02.AllowV02MetadataKey, "1")
		t.Setenv(dev.DevModeKey, "1")

		repo, state := createTestRepository(t, createTestStateWithPolicyUsingPersons)

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err := verifyEntry(testCtx, repo, state, nil, entry)
		assert.Nil(t, err)
	})

	t.Run("successful verification with higher threshold using v0.1 reference authorization", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicy)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Create authorization for this change
		// We're explicitly using the old type here to ensure policy
		// verification still works
		authorization, err := authorizationsv01.NewReferenceAuthorization(refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add authorization", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.Nil(t, err)
	})

	t.Run("successful verification with higher threshold using latest reference authorization", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicy)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Create authorization for this change
		// This uses the latest reference authorization version
		authorization, err := attestations.NewReferenceAuthorizationForCommit(refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add authorization", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.Nil(t, err)
	})

	t.Run("successful verification with higher threshold but using GitHub approval", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
		t.Setenv(tufv02.AllowV02MetadataKey, "1")

		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// This is using the jane.doe signer
		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Create authorization for this change using john.doe trusted as approver
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"john.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.Nil(t, err)
	})

	t.Run("unsuccessful verification with higher threshold but using GitHub approval due to invalid app key", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
		t.Setenv(tufv02.AllowV02MetadataKey, "1")

		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// This is using the jane.doe signer
		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Create authorization for this change using john.doe trusted as approver
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"john.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is NOT trusted in the root setup by
		// the policy state creator helper
		signer := setupSSHKeysForSigning(t, targets2KeyBytes, targets2PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("successful verification with higher threshold but using GitHub approval and reference authorization v0.2", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrustForMixedAttestations)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// This is the jane.doe principal
		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Approved by jill.doe
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"jill.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		// Add reference authorization for john.doe
		signer = setupSSHKeysForSigning(t, targets2KeyBytes, targets2PubKeyBytes)

		authorization, err := attestations.NewReferenceAuthorizationForCommit(refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		env, err = dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add authorization", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.Nil(t, err)
	})

	t.Run("unsuccessful verification with higher threshold but using GitHub approval from untrusted key and reference authorization v0.2", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrustForMixedAttestations)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// This is the jane.doe principal
		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Approved by jill.doe
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"jill.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is NOT trusted in the root setup by
		// the policy state creator helper
		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		// Add reference authorization for john.doe
		signer = setupSSHKeysForSigning(t, targets2KeyBytes, targets2PubKeyBytes)

		authorization, err := attestations.NewReferenceAuthorizationForCommit(refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		env, err = dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add authorization", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("unsuccessful verification with higher threshold but using GitHub approval", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Create approval for jill.doe -> NOT TRUSTED in this policy
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"jill.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("unsuccessful verification with higher threshold when a person signs reference authorization and uses GitHub approval", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrustForMixedAttestations)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Create approval for john.doe
		githubAppApproval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), commitTreeID.String(), []string{"john.doe"}, nil)
		if err != nil {
			t.Fatal(err)
		}

		// This signer for the GitHub app is trusted in the root setup by the
		// policy state creator helper
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(githubAppApproval)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppRoleName, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Add reference authorization for john.doe
		signer = setupSSHKeysForSigning(t, targets2KeyBytes, targets2PubKeyBytes)

		authorization, err := attestations.NewReferenceAuthorizationForCommit(refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		env, err = dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add authorization", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// We have an RSL signature from jane.doe, a GitHub approval from
		// john.doe and a reference authorization from john.doe
		// Insufficient to meet threshold 3
		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("successful verification with global threshold constraint", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithGlobalConstraintThreshold)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Create authorization for this change
		// This uses the latest reference authorization version
		authorization, err := attestations.NewReferenceAuthorizationForCommit(refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes) // this is trusted in the global constraint state creator

		env, err := dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add authorization", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.Nil(t, err)
	})

	t.Run("unsuccessful verification with global threshold constraint", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithGlobalConstraintThreshold)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		commitTreeID, err := repo.GetCommitTreeID(commitIDs[0])
		if err != nil {
			t.Fatal(err)
		}

		// Create authorization for this change
		// This uses the latest reference authorization version
		authorization, err := attestations.NewReferenceAuthorizationForCommit(refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		if err != nil {
			t.Fatal(err)
		}

		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes) // this is NOT trusted in the global constraint state creator

		env, err := dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add authorization", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("verify block force pushes rule for protected ref", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithGlobalConstraintBlockForcePushes)

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)

		entry := rsl.NewReferenceEntry(refName, commitIDs[1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Only one entry, this is fine
		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.Nil(t, err)

		// Add more entries
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// Still fine
		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.Nil(t, err)

		// Rewrite history altogether
		// Delete ref
		if err := repo.SetReference(refName, gitinterface.ZeroHash); err != nil {
			t.Fatal(err)
		}

		// Switch up the key for good measure
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, rootKeyBytes)

		entry = rsl.NewReferenceEntry(refName, commitIDs[1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, rootKeyBytes)
		entry.ID = entryID

		// Not fine
		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("verify block force pushes rule for unprotected ref", func(t *testing.T) {
		refName := "refs/heads/feature"
		repo, state := createTestRepository(t, createTestStateWithGlobalConstraintBlockForcePushes)

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)

		entry := rsl.NewReferenceEntry(refName, commitIDs[1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		// Only one entry, this is fine
		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.Nil(t, err)

		// Add more entries
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// Still fine
		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.Nil(t, err)

		// Rewrite history altogether
		// Delete ref
		if err := repo.SetReference(refName, gitinterface.ZeroHash); err != nil {
			t.Fatal(err)
		}

		// Switch up the key for good measure
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, rootKeyBytes)

		entry = rsl.NewReferenceEntry(refName, commitIDs[1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, rootKeyBytes)
		entry.ID = entryID

		// Still fine; this ref is not protected
		err = verifyEntry(testCtx, repo, state, currentAttestations, entry)
		assert.Nil(t, err)
	})

	t.Run("verify global rules applied from controller repository", func(t *testing.T) {
		controllerRepositoryLocation := t.TempDir()
		networkRepositoryLocation := t.TempDir()

		controllerRepository := gitinterface.CreateTestGitRepository(t, controllerRepositoryLocation, true)
		controllerState := createTestStateWithGlobalConstraintThreshold(t)
		controllerState.repository = controllerRepository

		networkRepository := gitinterface.CreateTestGitRepository(t, networkRepositoryLocation, false)
		networkState := createTestStateWithPolicy(t)
		networkState.repository = networkRepository

		signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

		controllerRootMetadata, err := controllerState.GetRootMetadata(false)
		require.Nil(t, err)
		err = controllerRootMetadata.EnableController()
		require.Nil(t, err)
		err = controllerRootMetadata.AddNetworkRepository("test", networkRepositoryLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())})
		require.Nil(t, err)
		controllerRootEnv, err := dsse.CreateEnvelope(controllerRootMetadata)
		require.Nil(t, err)
		controllerRootEnv, err = dsse.SignEnvelope(testCtx, controllerRootEnv, signer)
		require.Nil(t, err)
		controllerState.Metadata.RootEnvelope = controllerRootEnv
		err = controllerState.preprocess()
		require.Nil(t, err)
		err = controllerState.Commit(controllerRepository, "Initial policy\n", true, false)
		require.Nil(t, err)
		err = Apply(testCtx, controllerRepository, false)
		require.Nil(t, err)
		latestControllerEntry, err := rsl.GetLatestEntry(controllerRepository)
		require.Nil(t, err)
		controllerState.loadedEntry = latestControllerEntry.(rsl.ReferenceUpdaterEntry)

		networkRootMetadata, err := networkState.GetRootMetadata(false)
		require.Nil(t, err)
		err = networkRootMetadata.AddControllerRepository("controller", controllerRepositoryLocation, []tuf.Principal{tufv01.NewKeyFromSSLibKey(signer.MetadataKey())})
		require.Nil(t, err)
		networkRootEnv, err := dsse.CreateEnvelope(networkRootMetadata)
		require.Nil(t, err)
		networkRootEnv, err = dsse.SignEnvelope(testCtx, networkRootEnv, signer)
		require.Nil(t, err)
		networkState.Metadata.RootEnvelope = networkRootEnv
		networkTargetsMetadata, err := networkState.GetTargetsMetadata(TargetsRoleName, false)
		require.Nil(t, err)
		err = networkTargetsMetadata.AddPrincipal(tufv01.NewKeyFromSSLibKey(signer.MetadataKey()))
		require.Nil(t, err)
		networkTargetsEnv, err := dsse.CreateEnvelope(networkTargetsMetadata)
		require.Nil(t, err)
		networkTargetsEnv, err = dsse.SignEnvelope(testCtx, networkTargetsEnv, signer)
		require.Nil(t, err)
		networkState.Metadata.TargetsEnvelope = networkTargetsEnv
		err = networkState.Commit(networkRepository, "Initial policy\n", true, false)
		require.Nil(t, err)
		err = Apply(testCtx, networkRepository, false)
		require.Nil(t, err)

		err = rsl.PropagateChangesFromUpstreamRepository(networkRepository, controllerRepository, networkRootMetadata.GetPropagationDirectives(), false)
		require.Nil(t, err)

		networkState, err = LoadCurrentState(testCtx, networkRepository, PolicyRef)
		require.Nil(t, err)

		refName := "refs/heads/main" // this has threshold 1 in network repo but threshold 2 in controller repo

		currentAttestations, err := attestations.LoadCurrentAttestations(networkRepository)
		require.Nil(t, err)

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, networkRepository, refName, 1, gpgKeyBytes)

		commitTreeID, err := networkRepository.GetCommitTreeID(commitIDs[0])
		require.Nil(t, err)

		// Create authorization for this change
		// This uses the latest reference authorization version
		authorization, err := attestations.NewReferenceAuthorizationForCommit(refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		require.Nil(t, err)

		env, err := dsse.CreateEnvelope(authorization)
		require.Nil(t, err)
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		require.Nil(t, err)

		err = currentAttestations.SetReferenceAuthorization(networkRepository, env, refName, gitinterface.ZeroHash.String(), commitTreeID.String())
		require.Nil(t, err)
		err = currentAttestations.Commit(networkRepository, "Add authorization", true, false)
		require.Nil(t, err)

		currentAttestations, err = attestations.LoadCurrentAttestations(networkRepository)
		require.Nil(t, err)

		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, networkRepository, entry, gpgKeyBytes)
		entry.ID = entryID

		// We meet the threshold of with the reference authorization, so this should be successful
		err = verifyEntry(testCtx, networkRepository, networkState, currentAttestations, entry)
		assert.Nil(t, err)

		// Make another change without reference authorization
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, networkRepository, refName, 2, gpgKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, networkRepository, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyEntry(testCtx, networkRepository, networkState, currentAttestations, entry)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})
}

func TestVerifyTagEntry(t *testing.T) {
	t.Run("no tag specific policy", func(t *testing.T) {
		repo, policy := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		tagName := "v1"
		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1], gpgKeyBytes)

		entry = rsl.NewReferenceEntry(gitinterface.TagReferenceName(tagName), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err := verifyTagEntry(testCtx, repo, policy, nil, entry)
		assert.Nil(t, err)
	})

	t.Run("with tag specific policy", func(t *testing.T) {
		repo, policy := createTestRepository(t, createTestStateWithTagPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		tagName := "v1"
		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1], gpgKeyBytes)

		entry = rsl.NewReferenceEntry(gitinterface.TagReferenceName(tagName), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err := verifyTagEntry(testCtx, repo, policy, nil, entry)
		assert.Nil(t, err)
	})

	t.Run("with threshold tag specific policy", func(t *testing.T) {
		repo, policy := createTestRepository(t, createTestStateWithThresholdTagPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		tagName := "v1"
		tagRefName := "refs/tags/v1"

		// Create authorization for this change
		// This uses the latest reference authorization version
		// As this is for a tag, the target is the commit the tag points to,
		// taken from the RSL entry we just created for it
		authorization, err := attestations.NewReferenceAuthorizationForTag(tagRefName, gitinterface.ZeroHash.String(), entry.TargetID.String())
		if err != nil {
			t.Fatal(err)
		}

		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		env, err := dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, tagRefName, gitinterface.ZeroHash.String(), entry.TargetID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add authorization", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1], gpgKeyBytes)

		entry = rsl.NewReferenceEntry(gitinterface.TagReferenceName(tagName), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyTagEntry(testCtx, repo, policy, currentAttestations, entry)
		assert.Nil(t, err)
	})

	t.Run("with tag specific policy, unauthorized", func(t *testing.T) {
		repo, policy := createTestRepository(t, createTestStateWithTagPolicyForUnauthorizedTest)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		tagName := "v1"
		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1], gpgKeyBytes)

		entry = rsl.NewReferenceEntry(gitinterface.TagReferenceName(tagName), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err := verifyTagEntry(testCtx, repo, policy, nil, entry)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})

	t.Run("with threshold tag specific policy, unauthorized", func(t *testing.T) {
		repo, policy := createTestRepository(t, createTestStateWithThresholdTagPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		tagName := "v1"
		tagRefName := "refs/tags/v1"

		// Create authorization for this change
		// This uses the latest reference authorization version
		// As this is for a tag, the target is the commit the tag points to,
		// taken from the RSL entry we just created for it
		authorization, err := attestations.NewReferenceAuthorizationForTag(tagRefName, gitinterface.ZeroHash.String(), entry.TargetID.String())
		if err != nil {
			t.Fatal(err)
		}

		// The policy expects targets1Key but we're signing with targets2Key
		signer := setupSSHKeysForSigning(t, targets2KeyBytes, targets2PubKeyBytes)

		env, err := dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, tagRefName, gitinterface.ZeroHash.String(), entry.TargetID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add authorization", true, false); err != nil {
			t.Fatal(err)
		}

		currentAttestations, err = attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1], gpgKeyBytes)

		entry = rsl.NewReferenceEntry(gitinterface.TagReferenceName(tagName), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = verifyTagEntry(testCtx, repo, policy, currentAttestations, entry)
		assert.ErrorIs(t, err, ErrVerificationFailed)
	})
}

func TestGetCommits(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)

	refName := "refs/heads/main"

	// FIXME: this setup with RSL entries can be formalized using another
	// helper like createTestStateWithPolicy. The RSL could then also
	// incorporate policy changes and so on.
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgKeyBytes)
	firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
	firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
	firstEntry.ID = firstEntryID

	secondEntry := rsl.NewReferenceEntry(refName, commitIDs[4])
	secondEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, secondEntry, gpgKeyBytes)
	secondEntry.ID = secondEntryID

	expectedCommitIDs := []gitinterface.Hash{commitIDs[1], commitIDs[2], commitIDs[3], commitIDs[4]}

	sort.Slice(expectedCommitIDs, func(i, j int) bool {
		return expectedCommitIDs[i].String() < expectedCommitIDs[j].String()
	})

	commitIDs, err := getCommits(repo, secondEntry)
	assert.Nil(t, err)
	assert.Equal(t, expectedCommitIDs, commitIDs)
}

func TestStateVerifyNewState(t *testing.T) {
	t.Parallel()
	t.Run("valid policy transition", func(t *testing.T) {
		t.Parallel()
		currentPolicy := createTestStateWithOnlyRoot(t)
		newPolicy := createTestStateWithOnlyRoot(t)

		err := currentPolicy.VerifyNewState(testCtx, newPolicy)
		assert.Nil(t, err)
	})

	t.Run("invalid policy transition", func(t *testing.T) {
		t.Parallel()
		currentPolicy := createTestStateWithOnlyRoot(t)

		// Create invalid state
		signer := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

		key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())

		rootMetadata, err := InitializeRootMetadata(key)
		if err != nil {
			t.Fatal(err)
		}

		rootEnv, err := dsse.CreateEnvelope(rootMetadata)
		if err != nil {
			t.Fatal(err)
		}
		rootEnv, err = dsse.SignEnvelope(testCtx, rootEnv, signer)
		if err != nil {
			t.Fatal(err)
		}
		newPolicy := &State{
			Metadata: &StateMetadata{
				RootEnvelope:        rootEnv,
				DelegationEnvelopes: map[string]*sslibdsse.Envelope{},
			},
		}

		err = currentPolicy.VerifyNewState(testCtx, newPolicy)
		assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
	})
}
