// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/stretchr/testify/assert"
)

func TestVerifyRef(t *testing.T) {
	repo := createTestRepositoryWithPolicy(t, "")

	refName := "refs/heads/main"

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyBytes)
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	entryID := common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgKeyBytes)
	entry.ID = entryID

	tests := map[string]struct {
		target     string
		latestOnly bool
		err        error
	}{
		"absolute ref, not full": {
			target:     "refs/heads/main",
			latestOnly: true,
		},
		"absolute ref, full": {
			target:     "refs/heads/main",
			latestOnly: false,
		},
		"relative ref, not full": {
			target:     "main",
			latestOnly: true,
		},
		"relative ref, full": {
			target:     "main",
			latestOnly: false,
		},
		"unknown ref, full": {
			target:     "refs/heads/unknown",
			latestOnly: false,
			err:        rsl.ErrRSLEntryNotFound,
		},
	}

	for name, test := range tests {
		err := repo.VerifyRef(testCtx, test.target, test.latestOnly)
		if test.err != nil {
			assert.ErrorIs(t, err, test.err, fmt.Sprintf("unexpected error in test '%s'", name))
		} else {
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		}
	}

	// Add another commit
	common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyBytes)
	err := repo.VerifyRef(testCtx, refName, true)
	assert.ErrorIs(t, err, ErrRefStateDoesNotMatchRSL)
	err = repo.VerifyRef(testCtx, refName, false)
	assert.ErrorIs(t, err, ErrRefStateDoesNotMatchRSL)
}

func TestVerifyRefFromEntry(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	repo := createTestRepositoryWithPolicy(t, "")

	refName := "refs/heads/main"

	// Policy violation
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgUnauthorizedKeyBytes)
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	violatingEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgUnauthorizedKeyBytes)

	// No policy violation
	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyBytes)
	entry = rsl.NewReferenceEntry(refName, commitIDs[0])
	goodEntryID := common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgKeyBytes)

	// No policy violation (latest)
	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyBytes)
	entry = rsl.NewReferenceEntry(refName, commitIDs[0])
	common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgKeyBytes)

	tests := map[string]struct {
		target      string
		fromEntryID gitinterface.Hash
		err         error
	}{
		"absolute ref, from non-violating": {
			target:      "refs/heads/main",
			fromEntryID: goodEntryID,
		},
		"absolute ref, from violating": {
			target:      "refs/heads/main",
			fromEntryID: violatingEntryID,
			err:         policy.ErrUnauthorizedSignature,
		},
		"relative ref, from non-violating": {
			target:      "main",
			fromEntryID: goodEntryID,
		},
		"relative ref, from violating": {
			target:      "main",
			fromEntryID: violatingEntryID,
			err:         policy.ErrUnauthorizedSignature,
		},
		"unknown ref": {
			target:      "refs/heads/unknown",
			fromEntryID: gitinterface.ZeroHash,
			err:         rsl.ErrRSLEntryNotFound,
		},
	}

	for name, test := range tests {
		err := repo.VerifyRefFromEntry(testCtx, test.target, test.fromEntryID.String())
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
	assert.ErrorIs(t, err, policy.ErrUnauthorizedSignature)
}
