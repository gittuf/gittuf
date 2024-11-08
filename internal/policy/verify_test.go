// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"sort"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/stretchr/testify/assert"
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

	currentTip, err := VerifyRef(testCtx, repo, refName)
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

	currentTip, err := VerifyRefFull(testCtx, repo, refName)
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

	// Verification passes because it's from a non-violating state only
	currentTip, err := VerifyRefFromEntry(testCtx, repo, refName, entryID)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		err = VerifyRelativeForRef(testCtx, repo, entry, firstEntry, refName)
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

		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		err = VerifyRelativeForRef(testCtx, repo, entry, firstEntry, refName)
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

		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		invalidEntryIDs := []gitinterface.Hash{entryID}

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's still in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		invalidEntryIDs := []gitinterface.Hash{entryID}

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's still in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		// Send it into invalid state again
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		invalidLastGoodCommitID := commitIDs[len(commitIDs)-1]

		// Add more commits, change the number of commits to have different trees
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 4, gpgKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		// Skip the recovery entry as well
		annotation = rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID = common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID

		// No fix entry, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		err = VerifyRelativeForRef(testCtx, repo, entry, firstEntry, refName)
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

		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		err = VerifyRelativeForRef(testCtx, repo, entry, firstEntry, refName)
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

		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		invalidEntryIDs := []gitinterface.Hash{entryID}

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's still in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		invalidEntryIDs := []gitinterface.Hash{entryID}

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's still in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		// Send it into invalid state again
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		invalidLastGoodCommitID := commitIDs[len(commitIDs)-1]

		// Add more commits, change the number of commits to have different trees
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 4, gpgKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

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
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		// Skip the recovery entry as well
		annotation = rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID = common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
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

		err := VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.Nil(t, err)

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]gitinterface.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID

		// No fix entry, error out
		err = VerifyRelativeForRef(testCtx, repo, firstEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
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
		if err := currentAttestations.Commit(repo, "Add authorization", false); err != nil {
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
		if err := currentAttestations.Commit(repo, "Add authorization", false); err != nil {
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

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppKeys[0].ID(), refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", false); err != nil {
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

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppKeys[0].ID(), refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", false); err != nil {
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
		if err := currentAttestations.Commit(repo, "Add authorization", false); err != nil {
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

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppKeys[0].ID(), refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", false); err != nil {
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
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
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

		if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, env, "https://github.com", 1, state.githubAppKeys[0].ID(), refName, gitinterface.ZeroHash.String(), commitTreeID.String()); err != nil {
			t.Fatal(err)
		}
		if err := currentAttestations.Commit(repo, "Add GitHub pull request approval", false); err != nil {
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
		if err := currentAttestations.Commit(repo, "Add authorization", false); err != nil {
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
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
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
		if err := currentAttestations.Commit(repo, "Add authorization", false); err != nil {
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
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
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
		if err := currentAttestations.Commit(repo, "Add authorization", false); err != nil {
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
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
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
			RootPublicKeys:      []tuf.Principal{key},
			RootEnvelope:        rootEnv,
			DelegationEnvelopes: map[string]*sslibdsse.Envelope{},
		}

		err = currentPolicy.VerifyNewState(testCtx, newPolicy)
		assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
	})
}

func TestVerifier(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootPubKeyR := rootSigner.MetadataKey()
	rootPubKey := tufv01.NewKeyFromSSLibKey(rootPubKeyR)

	targetsSigner := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)
	targetsPubKeyR := targetsSigner.MetadataKey()
	targetsPubKey := tufv01.NewKeyFromSSLibKey(targetsPubKeyR)

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, "refs/heads/main", 1, gpgKeyBytes)
	commitID := commitIDs[0]
	tagID := common.CreateTestSignedTag(t, repo, "test-tag", commitID, gpgKeyBytes)

	attestation, err := dsse.CreateEnvelope(nil)
	if err != nil {
		t.Fatal(err)
	}
	attestation, err = dsse.SignEnvelope(testCtx, attestation, rootSigner)
	if err != nil {
		t.Fatal(err)
	}

	invalidAttestation, err := dsse.CreateEnvelope(nil)
	if err != nil {
		t.Fatal(err)
	}
	invalidAttestation, err = dsse.SignEnvelope(testCtx, invalidAttestation, targetsSigner)
	if err != nil {
		t.Fatal(err)
	}

	attestationWithTwoSigs, err := dsse.CreateEnvelope(nil)
	if err != nil {
		t.Fatal(err)
	}
	attestationWithTwoSigs, err = dsse.SignEnvelope(testCtx, attestationWithTwoSigs, rootSigner)
	if err != nil {
		t.Fatal(err)
	}
	attestationWithTwoSigs, err = dsse.SignEnvelope(testCtx, attestationWithTwoSigs, targetsSigner)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		principals  []tuf.Principal
		threshold   int
		gitObjectID gitinterface.Hash
		attestation *sslibdsse.Envelope

		expectedError error
	}{
		"commit, no attestation, valid key, threshold 1": {
			principals:  []tuf.Principal{gpgKey},
			threshold:   1,
			gitObjectID: commitID,
		},
		"commit, no attestation, valid key, threshold 2": {
			principals:    []tuf.Principal{gpgKey},
			threshold:     2,
			gitObjectID:   commitID,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"commit, attestation, valid key, threshold 1": {
			principals:  []tuf.Principal{gpgKey},
			threshold:   1,
			gitObjectID: commitID,
			attestation: attestation,
		},
		"commit, attestation, valid keys, threshold 2": {
			principals:  []tuf.Principal{gpgKey, rootPubKey},
			threshold:   2,
			gitObjectID: commitID,
			attestation: attestation,
		},
		"commit, invalid signed attestation, threshold 2": {
			principals:    []tuf.Principal{gpgKey, rootPubKey},
			threshold:     2,
			gitObjectID:   commitID,
			attestation:   invalidAttestation,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"commit, attestation, valid keys, threshold 3": {
			principals:  []tuf.Principal{gpgKey, rootPubKey, targetsPubKey},
			threshold:   3,
			gitObjectID: commitID,
			attestation: attestationWithTwoSigs,
		},
		"tag, no attestation, valid key, threshold 1": {
			principals:  []tuf.Principal{gpgKey},
			threshold:   1,
			gitObjectID: tagID,
		},
		"tag, no attestation, valid key, threshold 2": {
			principals:    []tuf.Principal{gpgKey},
			threshold:     2,
			gitObjectID:   tagID,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"tag, attestation, valid key, threshold 1": {
			principals:  []tuf.Principal{gpgKey},
			threshold:   1,
			gitObjectID: tagID,
			attestation: attestation,
		},
		"tag, attestation, valid keys, threshold 2": {
			principals:  []tuf.Principal{gpgKey, rootPubKey},
			threshold:   2,
			gitObjectID: tagID,
			attestation: attestation,
		},
		"tag, invalid signed attestation, threshold 2": {
			principals:    []tuf.Principal{gpgKey, rootPubKey},
			threshold:     2,
			gitObjectID:   tagID,
			attestation:   invalidAttestation,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"tag, attestation, valid keys, threshold 3": {
			principals:  []tuf.Principal{gpgKey, rootPubKey, targetsPubKey},
			threshold:   3,
			gitObjectID: tagID,
			attestation: attestationWithTwoSigs,
		},
	}

	for name, test := range tests {
		verifier := Verifier{
			repository: repo,
			name:       "test-verifier",
			principals: test.principals,
			threshold:  test.threshold,
		}

		_, err := verifier.Verify(testCtx, test.gitObjectID, test.attestation)
		if test.expectedError == nil {
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		} else {
			assert.ErrorIs(t, err, test.expectedError, fmt.Sprintf("incorrect error received in test '%s'", name))
		}
	}
}
