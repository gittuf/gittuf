// Copyright The gittuf Authors
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
	verifyopts "github.com/gittuf/gittuf/internal/repository/options/verify"
)

// ErrRefStateDoesNotMatchRSL is returned when a Git reference being verified
// does not have the same tip as identified in the latest RSL entry for the
// reference. This can happen for a number of reasons such as incorrectly
// modifying reference state away from what's recorded in the RSL to not
// creating an RSL entry for some new changes. Depending on the context, one
// resolution is to update the reference state to match the RSL entry, while
// another is to create a new RSL entry for the current state.
var ErrRefStateDoesNotMatchRSL = errors.New("Git reference's current state does not match latest RSL entry") //nolint:stylecheck

func (r *Repository) VerifyRef(ctx context.Context, refName string, opts ...verifyopts.Option) error {
	var (
		expectedTip gitinterface.Hash
		err         error
	)

	options := &verifyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Identifying absolute reference path...")
	refName, err = r.r.AbsoluteReference(refName)
	if err != nil {
		return err
	}

	// Track localRefName to check the expected tip as we may override refName
	localRefName := refName

	if options.RefNameOverride != "" {
		// remote ref name is different
		// We must consider RSL entries that have refNameOverride rather than
		// refName
		slog.Debug("Name of reference overridden to match remote reference name, identifying absolute reference path...")
		refNameOverride, err := r.r.AbsoluteReference(options.RefNameOverride)
		if err != nil {
			return err
		}

		refName = refNameOverride
	}

	slog.Debug(fmt.Sprintf("Verifying gittuf policies for '%s'", refName))

	if options.LatestOnly {
		expectedTip, err = policy.VerifyRef(ctx, r.r, refName)
	} else {
		expectedTip, err = policy.VerifyRefFull(ctx, r.r, refName)
	}
	if err != nil {
		return err
	}

	// To verify the tip, we _must_ use the localRefName
	slog.Debug("Verifying if tip of reference matches expected value from RSL...")
	if err := r.verifyRefTip(localRefName, expectedTip); err != nil {
		return err
	}

	slog.Debug("Verification successful!")
	return nil
}

func (r *Repository) VerifyRefFromEntry(ctx context.Context, refName, entryID string, opts ...verifyopts.Option) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	options := &verifyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	var err error

	slog.Debug("Identifying absolute reference path...")
	refName, err = r.r.AbsoluteReference(refName)
	if err != nil {
		return err
	}

	entryIDHash, err := gitinterface.NewHash(entryID)
	if err != nil {
		return err
	}

	// Track localRefName to check the expected tip as we may override refName
	localRefName := refName

	if options.RefNameOverride != "" {
		// remote ref name is different
		// We must consider RSL entries that have refNameOverride rather than
		// refName
		slog.Debug("Name of reference overridden to match remote reference name, identifying absolute reference path...")
		refNameOverride, err := r.r.AbsoluteReference(options.RefNameOverride)
		if err != nil {
			return err
		}

		refName = refNameOverride
	}

	slog.Debug(fmt.Sprintf("Verifying gittuf policies for '%s' from entry '%s'", refName, entryID))
	expectedTip, err := policy.VerifyRefFromEntry(ctx, r.r, refName, entryIDHash)
	if err != nil {
		return err
	}

	// To verify the tip, we _must_ use the localRefName
	slog.Debug("Verifying if tip of reference matches expected value from RSL...")
	if err := r.verifyRefTip(localRefName, expectedTip); err != nil {
		return err
	}

	slog.Debug("Verification successful!")
	return nil
}

// verifyRefTip inspects the specified reference in the local repository to
// check if it points to the expected Git object.
func (r *Repository) verifyRefTip(target string, expectedTip gitinterface.Hash) error {
	refTip, err := r.r.GetReference(target)
	if err != nil {
		return err
	}

	if !refTip.Equal(expectedTip) {
		return ErrRefStateDoesNotMatchRSL
	}

	return nil
}
