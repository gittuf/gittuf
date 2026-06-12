// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"fmt"
	"testing"

	attestopts "github.com/gittuf/gittuf/experimental/gittuf/options/attest"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	verifyopts "github.com/gittuf/gittuf/experimental/gittuf/options/verify"
	verifymergeableopts "github.com/gittuf/gittuf/experimental/gittuf/options/verifymergeable"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestVerifyRef(t *testing.T) {
	repo := createTestRepositoryWithPolicy(t, "")

	refName := "refs/heads/main"
	remoteRefName := "refs/heads/not-main"

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyBytes)
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	entryID := common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgKeyBytes)
	entry.ID = entryID

	// Add one entry for a different remote ref name
	entry = rsl.NewReferenceEntry(remoteRefName, commitIDs[0])
	entryID = common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgKeyBytes)
	entry.ID = entryID

	tests := map[string]struct {
		localRefName  string
		remoteRefName string
		latestOnly    bool
		err           error
	}{
		"absolute ref, not full": {
			localRefName: refName,
			latestOnly:   true,
		},
		"absolute ref, full": {
			localRefName: refName,
			latestOnly:   false,
		},
		"relative ref, not full": {
			localRefName: "main",
			latestOnly:   true,
		},
		"relative ref, full": {
			localRefName: "main",
			latestOnly:   false,
		},
		"unknown ref, full": {
			localRefName: "refs/heads/unknown",
			latestOnly:   false,
			err:          rsl.ErrRSLEntryNotFound,
		},
		"different local and remote ref names, not full": {
			localRefName:  refName,
			remoteRefName: remoteRefName,
			latestOnly:    true,
		},
		"different local and remote ref names, full": {
			localRefName:  refName,
			remoteRefName: remoteRefName,
			latestOnly:    false,
		},
		"unknown remote ref, full": {
			localRefName:  refName,
			remoteRefName: "refs/heads/unknown",
			latestOnly:    false,
			err:           rsl.ErrRSLEntryNotFound,
		},
	}

	for name, test := range tests {
		options := []verifyopts.Option{verifyopts.WithOverrideRefName(test.remoteRefName)}
		if test.latestOnly {
			options = append(options, verifyopts.WithLatestOnly())
		}

		err := repo.VerifyRef(testCtx, test.localRefName, options...)
		if test.err != nil {
			assert.ErrorIs(t, err, test.err, fmt.Sprintf("unexpected error in test '%s'", name))
		} else {
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		}
	}

	// Add another commit
	common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyBytes)
	err := repo.VerifyRef(testCtx, refName, verifyopts.WithLatestOnly())
	assert.ErrorIs(t, err, ErrRefStateDoesNotMatchRSL)
	err = repo.VerifyRef(testCtx, refName, verifyopts.WithLatestOnly())
	assert.ErrorIs(t, err, ErrRefStateDoesNotMatchRSL)
}

func TestVerifyRefFromEntry(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	repo := createTestRepositoryWithPolicy(t, "")

	refName := "refs/heads/main"
	remoteRefName := "refs/heads/not-main"

	// Policy violation
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgUnauthorizedKeyBytes)
	// Violation for refName
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	violatingEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgUnauthorizedKeyBytes)
	// Violation for remoteRefName
	entry = rsl.NewReferenceEntry(remoteRefName, commitIDs[0])
	violatingRemoteRefNameEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgUnauthorizedKeyBytes)

	// No policy violation for refName
	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyBytes)
	// refName
	entry = rsl.NewReferenceEntry(refName, commitIDs[0])
	goodEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgKeyBytes)
	// remoteRefName
	entry = rsl.NewReferenceEntry(remoteRefName, commitIDs[0])
	goodRemoteRefNameEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgKeyBytes)

	// No policy violation for refName (what we verify)
	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyBytes)
	entry = rsl.NewReferenceEntry(refName, commitIDs[0])
	common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgKeyBytes)
	// No policy violation for remoteRefName (what we verify)
	entry = rsl.NewReferenceEntry(remoteRefName, commitIDs[0])
	common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgKeyBytes)

	tests := map[string]struct {
		localRefName  string
		remoteRefName string
		fromEntryID   gitinterface.Hash
		err           error
	}{
		"absolute ref, from non-violating": {
			localRefName: "refs/heads/main",
			fromEntryID:  goodEntryID,
		},
		"absolute ref, from violating": {
			localRefName: "refs/heads/main",
			fromEntryID:  violatingEntryID,
			err:          policy.ErrVerificationFailed,
		},
		"relative ref, from non-violating": {
			localRefName: "main",
			fromEntryID:  goodEntryID,
		},
		"relative ref, from violating": {
			localRefName: "main",
			fromEntryID:  violatingEntryID,
			err:          policy.ErrVerificationFailed,
		},
		"unknown ref": {
			localRefName: "refs/heads/unknown",
			fromEntryID:  gitinterface.ZeroHash,
			err:          rsl.ErrRSLEntryNotFound,
		},
		"different local and remote ref names, from non-violating": {
			localRefName:  refName,
			remoteRefName: remoteRefName,
			fromEntryID:   goodRemoteRefNameEntryID,
		},
		"different local and remote ref names, from violating": {
			localRefName:  refName,
			remoteRefName: remoteRefName,
			fromEntryID:   violatingRemoteRefNameEntryID,
		},
	}

	for name, test := range tests {
		err := repo.VerifyRefFromEntry(testCtx, test.localRefName, test.fromEntryID.String(), verifyopts.WithOverrideRefName(test.remoteRefName))
		if test.err != nil {
			assert.ErrorIs(t, err, test.err, fmt.Sprintf("unexpected error in test '%s'", name))
		} else {
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		}
	}

	// Add another commit
	common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyBytes)

	// Verifying from only good entry tells us ref does not match RSL
	err := repo.VerifyRefFromEntry(testCtx, refName, goodEntryID.String())
	assert.ErrorIs(t, err, ErrRefStateDoesNotMatchRSL)

	// Verifying from violating entry tells us unauthorized signature
	err = repo.VerifyRefFromEntry(testCtx, refName, violatingEntryID.String())
	assert.ErrorIs(t, err, policy.ErrVerificationFailed)
}

func TestVerifyMergeable(t *testing.T) {
	targetRef := "refs/heads/main"
	featureRef := "refs/heads/feature"

	t.Run("not mergeable without approval", func(t *testing.T) {
		repo := createTestRepositoryWithPolicy(t, "")

		treeBuilder := gitinterface.NewTreeBuilder(repo.r)
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		baseCommitID, err := repo.r.Commit(emptyTreeID, targetRef, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		if err := repo.r.SetReference(featureRef, baseCommitID); err != nil {
			t.Fatal(err)
		}
		common.AddNTestCommitsToSpecifiedRef(t, repo.r, featureRef, 1, gpgUnauthorizedKeyBytes)
		if err := repo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		needsSignature, err := repo.VerifyMergeable(testCtx, targetRef, featureRef)
		assert.ErrorIs(t, err, policy.ErrVerificationFailed)
		assert.False(t, needsSignature)
	})

	t.Run("mergeable with approval", func(t *testing.T) {
		repo := createTestRepositoryWithPolicy(t, "")

		treeBuilder := gitinterface.NewTreeBuilder(repo.r)
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		baseCommitID, err := repo.r.Commit(emptyTreeID, targetRef, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		if err := repo.r.SetReference(featureRef, baseCommitID); err != nil {
			t.Fatal(err)
		}
		common.AddNTestCommitsToSpecifiedRef(t, repo.r, featureRef, 1, gpgUnauthorizedKeyBytes)
		if err := repo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		gpg.SetupTestGPGHomeDir(t, gpgKeyBytes)
		approverSigner, err := gpg.NewSignerFromKeyID("157507bbe151e378ce8126c1dcfe043cdd2db96e")
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.AddReferenceAuthorization(testCtx, approverSigner, targetRef, featureRef, false, attestopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		needsSignature, err := repo.VerifyMergeable(testCtx, targetRef, featureRef)
		assert.Nil(t, err)
		assert.False(t, needsSignature)
	})

	t.Run("mergeable with approval and feature RSL bypass", func(t *testing.T) {
		repo := createTestRepositoryWithPolicy(t, "")

		treeBuilder := gitinterface.NewTreeBuilder(repo.r)
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		baseCommitID, err := repo.r.Commit(emptyTreeID, targetRef, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.RecordRSLEntryForReference(testCtx, targetRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		if err := repo.r.SetReference(featureRef, baseCommitID); err != nil {
			t.Fatal(err)
		}
		common.AddNTestCommitsToSpecifiedRef(t, repo.r, featureRef, 1, gpgUnauthorizedKeyBytes)
		if err := repo.RecordRSLEntryForReference(testCtx, featureRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		gpg.SetupTestGPGHomeDir(t, gpgKeyBytes)
		approverSigner, err := gpg.NewSignerFromKeyID("157507bbe151e378ce8126c1dcfe043cdd2db96e")
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.AddReferenceAuthorization(testCtx, approverSigner, targetRef, featureRef, false, attestopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		needsSignature, err := repo.VerifyMergeable(testCtx, targetRef, featureRef, verifymergeableopts.WithBypassRSLForFeatureRef())
		assert.Nil(t, err)
		assert.False(t, needsSignature)
	})
}
