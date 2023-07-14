package repository

import (
	"context"
	"fmt"
	"testing"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestVerifyRef(t *testing.T) {
	repo := createTestRepositoryWithPolicy(t)

	if err := repo.r.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName("refs/heads/main"), plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	entry := rsl.NewEntry("refs/heads/main", plumbing.ZeroHash)
	entryID := common.CreateTestRSLEntryCommit(t, repo.r, entry)
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
