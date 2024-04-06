// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"fmt"
	"sort"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jonboulle/clockwork"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

// FIXME: the verification tests do not check for expected failures. More
// broadly, we need to rework the test setup here starting with
// createTestRepository and the state creation helpers.

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

func TestVerifyRef(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)

	currentTip, err := VerifyRef(context.Background(), repo, refName)
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

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)

	currentTip, err := VerifyRefFull(context.Background(), repo, refName)
	assert.Nil(t, err)
	assert.Equal(t, commitIDs[0], currentTip)
}

func TestVerifyRefFromEntry(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"

	if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

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

func TestVerifyRelativeForRef(t *testing.T) {
	t.Run("no recovery", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, entry, policyEntry, refName)
		assert.ErrorIs(t, err, rsl.ErrRSLEntryNotFound)
	})

	t.Run("with recovery, commit-same, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Fix using the known-good commit
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), validCommitID)); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]plumbing.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, commit-same, recovered by unauthorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Fix using the known-good commit
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), validCommitID)); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]plumbing.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgUnauthorizedKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// No error anymore
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, tree-same, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Fix using the known-good commit's tree
		ref, err := repo.Reference(plumbing.ReferenceName(refName), true)
		if err != nil {
			t.Fatal(err)
		}
		validCommit, err := repo.CommitObject(validCommitID)
		if err != nil {
			t.Fatal(err)
		}
		newCommit := gitinterface.CreateCommitObject(testGitConfig, validCommit.TreeHash, []plumbing.Hash{commitIDs[0]}, "Revert invalid commit", testClock)
		newCommit = common.SignTestCommit(t, repo, newCommit, gpgKeyBytes)
		newCommitID, err := gitinterface.ApplyCommit(repo, newCommit, ref)
		if err != nil {
			t.Fatal(err)
		}

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]plumbing.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, newCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, tree-same, recovered by unauthorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Fix using the known-good commit's tree
		ref, err := repo.Reference(plumbing.ReferenceName(refName), true)
		if err != nil {
			t.Fatal(err)
		}
		validCommit, err := repo.CommitObject(validCommitID)
		if err != nil {
			t.Fatal(err)
		}
		newCommit := gitinterface.CreateCommitObject(testGitConfig, validCommit.TreeHash, []plumbing.Hash{commitIDs[0]}, "Revert invalid commit", testClock)
		newCommit = common.SignTestCommit(t, repo, newCommit, gpgUnauthorizedKeyBytes)
		newCommitID, err := gitinterface.ApplyCommit(repo, newCommit, ref)
		if err != nil {
			t.Fatal(err)
		}

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]plumbing.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgUnauthorizedKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, newCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// No error anymore
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, commit-same, multiple invalid entries, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		invalidEntryIDs := []plumbing.Hash{entryID}

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's still in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		invalidEntryIDs = append(invalidEntryIDs, entryID)

		// Fix using the known-good commit
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), validCommitID)); err != nil {
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
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, commit-same, unskipped invalid entries, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		invalidEntryIDs := []plumbing.Hash{entryID}

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's still in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Fix using the known-good commit
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), validCommitID)); err != nil {
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
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrInvalidEntryNotSkipped)
	})

	t.Run("with recovery, commit-same, recovered by authorized user, last good state is due to recovery", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Fix using the known-good commit
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), validCommitID)); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]plumbing.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		// Send it into invalid state again
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Fix using the known-good commit
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), validCommitID)); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation = rsl.NewAnnotationEntry([]plumbing.Hash{entryID}, true, "invalid entry")
		annotationID = common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, validCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)
	})

	t.Run("with recovery, error because recovery goes back too far, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		// Add some commits
		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		invalidLastGoodCommitID := commitIDs[len(commitIDs)-1]

		// Add more commits, change the number of commits to have different trees
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 4, gpgKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Fix using the invalid last good commit
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), invalidLastGoodCommitID)); err != nil {
			t.Fatal(err)
		}
		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]plumbing.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to invalid last good commit
		entry = rsl.NewReferenceEntry(refName, invalidLastGoodCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
	})

	t.Run("with recovery but recovered entry is also skipped, tree-same, recovered by authorized user", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		validCommitID := commitIDs[0] // track this for later
		commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgUnauthorizedKeyBytes)
		entry = rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgUnauthorizedKeyBytes)
		entry.ID = entryID

		// It's in an invalid state right now, error out
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)

		// Fix using the known-good commit's tree
		ref, err := repo.Reference(plumbing.ReferenceName(refName), true)
		if err != nil {
			t.Fatal(err)
		}
		validCommit, err := repo.CommitObject(validCommitID)
		if err != nil {
			t.Fatal(err)
		}
		newCommit := gitinterface.CreateCommitObject(testGitConfig, validCommit.TreeHash, []plumbing.Hash{commitIDs[0]}, "Revert invalid commit", testClock)
		newCommit = common.SignTestCommit(t, repo, newCommit, gpgKeyBytes)
		newCommitID, err := gitinterface.ApplyCommit(repo, newCommit, ref)
		if err != nil {
			t.Fatal(err)
		}

		// Create a skip annotation for the invalid entry
		annotation := rsl.NewAnnotationEntry([]plumbing.Hash{entryID}, true, "invalid entry")
		annotationID := common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		// Create a new entry moving branch back to valid commit
		entry = rsl.NewReferenceEntry(refName, newCommitID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		// No error anymore
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.Nil(t, err)

		// Skip the recovery entry as well
		annotation = rsl.NewAnnotationEntry([]plumbing.Hash{entryID}, true, "invalid entry")
		annotationID = common.CreateTestRSLAnnotationEntryCommit(t, repo, annotation, gpgKeyBytes)
		annotation.ID = annotationID
		err = VerifyRelativeForRef(context.Background(), repo, policyEntry, nil, policyEntry, entry, refName)
		assert.ErrorIs(t, err, ErrUnauthorizedSignature)
	})
}

func TestVerifyCommit(t *testing.T) {
	repo, _ := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"
	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
	entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
	entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
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
	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

	expectedStatus = map[string]string{commitIDs[0].String(): unableToFindPolicyMessage}
	status = VerifyCommit(testCtx, repo, commitIDs[0].String())
	assert.Equal(t, expectedStatus, status)
}

func TestVerifyTag(t *testing.T) {
	t.Run("normal test", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		tagName := "v1"
		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1], gpgKeyBytes)

		expectedStatus := map[string]string{tagID.String(): unableToFindRSLEntryMessage}
		status := VerifyTag(context.Background(), repo, []string{tagID.String()})
		assert.Equal(t, expectedStatus, status)

		entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
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
	})

	t.Run("tag verification with changed tag", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		tagName := "v1"
		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1], gpgKeyBytes)

		entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-2], gpgKeyBytes)

		// Use tag ID
		expectedStatus := map[string]string{tagID.String(): "verifying RSL entry failed, tag reference set to unexpected target"}
		status := VerifyTag(context.Background(), repo, []string{tagID.String()})
		assert.Equal(t, expectedStatus, status)
	})

	t.Run("tag verification with multiple RSL entries", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithPolicy)
		refName := "refs/heads/main"

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		tagName := "v1"
		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1], gpgKeyBytes)

		entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		expectedStatus := map[string]string{tagID.String(): multipleTagRSLEntriesFoundMessage}
		status := VerifyTag(context.Background(), repo, []string{tagID.String()})
		assert.Equal(t, expectedStatus, status)
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

		err := verifyEntry(context.Background(), repo, state, nil, entry)
		assert.Nil(t, err)
	})

	t.Run("successful verification with higher threshold", func(t *testing.T) {
		repo, state := createTestRepository(t, createTestStateWithThresholdPolicy)

		currentAttestations, err := attestations.LoadCurrentAttestations(repo)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)

		// Create authorization for this change
		// TODO: determine authorization format, this requires the target commit
		// ID instead of something that could be determined ahead of time like
		// the tree ID
		authorization, err := attestations.NewReferenceAuthorization(refName, plumbing.ZeroHash.String(), commitIDs[0].String())
		if err != nil {
			t.Fatal(err)
		}

		signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targets1KeyBytes) //nolint:staticcheck
		if err != nil {
			t.Fatal(err)
		}
		env, err := dsse.CreateEnvelope(authorization)
		if err != nil {
			t.Fatal(err)
		}
		env, err = dsse.SignEnvelope(testCtx, env, signer)
		if err != nil {
			t.Fatal(err)
		}

		if err := currentAttestations.SetReferenceAuthorization(repo, env, refName, plumbing.ZeroHash.String(), commitIDs[0].String()); err != nil {
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

		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 3, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[len(commitIDs)-1])
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		tagName := "v1"
		tagID := common.CreateTestSignedTag(t, repo, tagName, commitIDs[len(commitIDs)-1], gpgKeyBytes)

		entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err := verifyTagEntry(context.Background(), repo, policy, entry)
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

		entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entry.ID = entryID

		err := verifyTagEntry(context.Background(), repo, policy, entry)
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

		entry = rsl.NewReferenceEntry(string(plumbing.NewTagReferenceName(tagName)), tagID)
		entryID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
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
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 5, gpgKeyBytes)
	firstEntry := rsl.NewReferenceEntry(refName, commitIDs[0])
	firstEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, firstEntry, gpgKeyBytes)
	firstEntry.ID = firstEntryID

	secondEntry := rsl.NewReferenceEntry(refName, commitIDs[4])
	secondEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo, secondEntry, gpgKeyBytes)
	secondEntry.ID = secondEntryID

	expectedCommitIDs := []plumbing.Hash{commitIDs[1], commitIDs[2], commitIDs[3], commitIDs[4]}
	expectedCommits := make([]*object.Commit, 0, len(expectedCommitIDs))
	for _, commitID := range expectedCommitIDs {
		commit, err := gitinterface.GetCommit(repo, commitID)
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
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 2, gpgKeyBytes)
	entries := []*rsl.ReferenceEntry{}
	for _, commitID := range commitIDs {
		entry := rsl.NewReferenceEntry(refName, commitID)
		entryID := common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
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
		signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targets1KeyBytes) //nolint:staticcheck
		if err != nil {
			t.Fatal(err)
		}

		key, err := tuf.LoadKeyFromBytes(targets1PubKeyBytes)
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
		assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
	})
}

func TestVerifier(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootPubKey, err := tuf.LoadKeyFromBytes(rootPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	targetsPubKey, err := tuf.LoadKeyFromBytes(targets1PubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	targetsSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targets1KeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	commit := gitinterface.CreateCommitObject(common.TestGitConfig, gitinterface.EmptyTree(), []plumbing.Hash{plumbing.ZeroHash}, "Test commit", common.TestClock)
	commit = common.SignTestCommit(t, repo, commit, gpgKeyBytes)
	// We need to do this because tag expects a valid target object
	commitID, err := gitinterface.WriteCommit(repo, commit)
	if err != nil {
		t.Fatal(err)
	}
	commit, err = repo.CommitObject(commitID) // FIXME: gitinterface.GetCommit
	if err != nil {
		t.Fatal(err)
	}

	tag := gitinterface.CreateTagObject(common.TestGitConfig, commit, "test-tag", "test-tag", common.TestClock)
	tag = common.SignTestTag(t, repo, tag, gpgKeyBytes)

	attestation, err := dsse.CreateEnvelope(nil)
	if err != nil {
		t.Fatal(err)
	}
	attestation, err = dsse.SignEnvelope(context.Background(), attestation, rootSigner)
	if err != nil {
		t.Fatal(err)
	}

	invalidAttestation, err := dsse.CreateEnvelope(nil)
	if err != nil {
		t.Fatal(err)
	}
	invalidAttestation, err = dsse.SignEnvelope(context.Background(), invalidAttestation, targetsSigner)
	if err != nil {
		t.Fatal(err)
	}

	attestationWithTwoSigs, err := dsse.CreateEnvelope(nil)
	if err != nil {
		t.Fatal(err)
	}
	attestationWithTwoSigs, err = dsse.SignEnvelope(context.Background(), attestationWithTwoSigs, rootSigner)
	if err != nil {
		t.Fatal(err)
	}
	attestationWithTwoSigs, err = dsse.SignEnvelope(context.Background(), attestationWithTwoSigs, targetsSigner)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		keys          []*tuf.Key
		threshold     int
		gitObject     object.Object
		attestation   *sslibdsse.Envelope
		expectedError error
	}{
		"commit, no attestation, valid key, threshold 1": {
			keys:      []*tuf.Key{gpgKey},
			threshold: 1,
			gitObject: commit,
		},
		"commit, no attestation, valid key, threshold 2": {
			keys:          []*tuf.Key{gpgKey},
			threshold:     2,
			gitObject:     commit,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"commit, attestation, valid key, threshold 1": {
			keys:        []*tuf.Key{gpgKey},
			threshold:   1,
			gitObject:   commit,
			attestation: attestation,
		},
		"commit, attestation, valid keys, threshold 2": {
			keys:        []*tuf.Key{gpgKey, rootPubKey},
			threshold:   2,
			gitObject:   commit,
			attestation: attestation,
		},
		"commit, invalid signed attestation, threshold 2": {
			keys:          []*tuf.Key{gpgKey, rootPubKey},
			threshold:     2,
			gitObject:     commit,
			attestation:   invalidAttestation,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"commit, attestation, valid keys, threshold 3": {
			keys:        []*tuf.Key{gpgKey, rootPubKey, targetsPubKey},
			threshold:   3,
			gitObject:   commit,
			attestation: attestationWithTwoSigs,
		},
		"tag, no attestation, valid key, threshold 1": {
			keys:      []*tuf.Key{gpgKey},
			threshold: 1,
			gitObject: tag,
		},
		"tag, no attestation, valid key, threshold 2": {
			keys:          []*tuf.Key{gpgKey},
			threshold:     2,
			gitObject:     tag,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"tag, attestation, valid key, threshold 1": {
			keys:        []*tuf.Key{gpgKey},
			threshold:   1,
			gitObject:   tag,
			attestation: attestation,
		},
		"tag, attestation, valid keys, threshold 2": {
			keys:        []*tuf.Key{gpgKey, rootPubKey},
			threshold:   2,
			gitObject:   tag,
			attestation: attestation,
		},
		"tag, invalid signed attestation, threshold 2": {
			keys:          []*tuf.Key{gpgKey, rootPubKey},
			threshold:     2,
			gitObject:     tag,
			attestation:   invalidAttestation,
			expectedError: ErrVerifierConditionsUnmet,
		},
		"tag, attestation, valid keys, threshold 3": {
			keys:        []*tuf.Key{gpgKey, rootPubKey, targetsPubKey},
			threshold:   3,
			gitObject:   tag,
			attestation: attestationWithTwoSigs,
		},
	}

	for name, test := range tests {
		verifier := Verifier{name: "test-verifier", keys: test.keys, threshold: test.threshold}
		err := verifier.Verify(context.Background(), test.gitObject, test.attestation)
		if test.expectedError == nil {
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		} else {
			assert.ErrorIs(t, err, test.expectedError, fmt.Sprintf("incorrect error received in test '%s'", name))
		}
	}
}
