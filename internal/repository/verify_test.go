// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestVerifyRef(t *testing.T) {
	repo := createTestRepositoryWithPolicy(t, "")

	refName := "refs/heads/main"
	if err := repo.r.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

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
		err := repo.VerifyRef(context.Background(), test.target, test.latestOnly, "")
		if test.err != nil {
			assert.ErrorIs(t, err, test.err, fmt.Sprintf("unexpected error in test '%s'", name))
		} else {
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		}
	}

	// Add another commit
	common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyBytes)
	err := repo.VerifyRef(context.Background(), refName, true, "")
	assert.ErrorIs(t, err, ErrRefStateDoesNotMatchRSL)
	err = repo.VerifyRef(context.Background(), refName, false, "")
	assert.ErrorIs(t, err, ErrRefStateDoesNotMatchRSL)
}
