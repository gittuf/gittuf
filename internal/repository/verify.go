// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/go-git/go-git/v5/plumbing"
)

// ErrRefStateDoesNotMatchRSL is returned when a Git reference being verified
// does not have the same tip as identified in the latest RSL entry for the
// reference. This can happen for a number of reasons such as incorrectly
// modifying reference state away from what's recorded in the RSL to not
// creating an RSL entry for some new changes. Depending on the context, one
// resolution is to update the reference state to match the RSL entry, while
// another is to create a new RSL entry for the current state.
var ErrRefStateDoesNotMatchRSL = errors.New("Git reference's current state does not match latest RSL entry") //nolint:stylecheck

func (r *Repository) VerifyRef(ctx context.Context, target string, latestOnly bool) error {
	var (
		expectedTip plumbing.Hash
		err         error
	)

	slog.Debug("Identifying absolute reference path...")
	target, err = gitinterface.AbsoluteReference(r.r, target)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Verifying gittuf policies for '%s'", target))

	if latestOnly {
		expectedTip, err = policy.VerifyRef(ctx, r.r, target)
	} else {
		expectedTip, err = policy.VerifyRefFull(ctx, r.r, target)
	}
	if err != nil {
		return err
	}

	slog.Debug("Verifying if tip of reference matches expected value from RSL...")
	if err := r.verifyRefTip(target, expectedTip); err != nil {
		return err
	}

	slog.Debug("Verification successful!")
	return nil
}

func (r *Repository) VerifyRefFromEntry(ctx context.Context, target, entryID string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	var err error

	slog.Debug("Identifying absolute reference path...")
	target, err = gitinterface.AbsoluteReference(r.r, target)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Verifying gittuf policies for '%s' from entry '%s'", target, entryID))
	expectedTip, err := policy.VerifyRefFromEntry(ctx, r.r, target, plumbing.NewHash(entryID))
	if err != nil {
		return err
	}

	slog.Debug("Verifying if tip of reference matches expected value from RSL...")
	if err := r.verifyRefTip(target, expectedTip); err != nil {
		return err
	}

	slog.Debug("Verification successful!")
	return nil
}

func (r *Repository) verifyRefTip(target string, expectedTip plumbing.Hash) error {
	ref, err := r.r.Reference(plumbing.ReferenceName(target), true)
	if err != nil {
		return err
	}

	if ref.Hash() != expectedTip {
		return ErrRefStateDoesNotMatchRSL
	}

	return nil
}
