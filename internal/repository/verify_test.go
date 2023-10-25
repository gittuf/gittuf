// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	"github.com/stretchr/testify/assert"
)

const gpgKeyName = "gpg-privkey.asc"

func TestVerifyRef(t *testing.T) {
	repo := createTestRepositoryWithPolicy(t, "")

	refName := "refs/heads/main"
	if err := repo.r.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo.r, refName, 1, gpgKeyName)
	entry := rsl.NewReferenceEntry(refName, commitIDs[0])
	entryID := common.CreateTestRSLReferenceEntryCommit(t, repo.r, entry, gpgKeyName)
	entry.ID = entryID

	tests := map[string]struct {
		target string
		full   bool
		err    error
	}{
		"absolute ref, not full": {
			target: "refs/heads/main",
			full:   false,
		},
		"absolute ref, full": {
			target: "refs/heads/main",
			full:   true,
		},
		"relative ref, not full": {
			target: "main",
			full:   false,
		},
		"relative ref, full": {
			target: "main",
			full:   true,
		},
		"unknown ref, full": {
			target: "refs/heads/unknown",
			full:   true,
			err:    rsl.ErrRSLEntryNotFound,
		},
	}

	for name, test := range tests {
		err := repo.VerifyRef(context.Background(), test.target, test.full)
		if test.err != nil {
			assert.ErrorIs(t, err, test.err, fmt.Sprintf("unexpected error in test '%s'", name))
		} else {
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		}
	}
}
