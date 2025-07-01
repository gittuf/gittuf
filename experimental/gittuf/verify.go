// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	verifyopts "github.com/gittuf/gittuf/experimental/gittuf/options/verify"
	verifymergeableopts "github.com/gittuf/gittuf/experimental/gittuf/options/verifymergeable"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
)

// ErrRefStateDoesNotMatchRSL is returned when a Git reference being verified
// does not have the same tip as identified in the latest RSL entry for the
// reference. This can happen for a number of reasons such as incorrectly
// modifying reference state away from what's recorded in the RSL to not
// creating an RSL entry for some new changes. Depending on the context, one
// resolution is to update the reference state to match the RSL entry, while
// another is to create a new RSL entry for the current state.
var ErrRefStateDoesNotMatchRSL = errors.New("current state of Git reference does not match latest RSL entry")

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

	verifier := policy.NewPolicyVerifier(r.r)

	if options.LatestOnly {
		expectedTip, err = verifier.VerifyRef(ctx, refName)
	} else {
		expectedTip, err = verifier.VerifyRefFull(ctx, refName)
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
	verifier := policy.NewPolicyVerifier(r.r)
	expectedTip, err := verifier.VerifyRefFromEntry(ctx, refName, entryIDHash)
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

// VerifyMergeable checks if the targetRef can be updated to reflect the changes
// in featureRef. It checks if sufficient authorizations / approvals exist for
// the merge to happen, indicated by the error being nil. Additionally, a
// boolean value is also returned that indicates whether a final authorized
// signature is still necessary via the RSL entry for the merge.
//
// Summary of return combinations:
// (false, err) -> merge is not possible
// (false, nil) -> merge is possible and can be performed by anyone
// (true,  nil) -> merge is possible but it MUST be performed by an authorized
// person for the rule, i.e., an authorized person must sign the merge's RSL
// entry
func (r *Repository) VerifyMergeable(ctx context.Context, targetRef, featureRef string, opts ...verifymergeableopts.Option) (bool, error) {
	var err error

	options := &verifymergeableopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Identifying absolute reference paths...")
	targetRef, err = r.r.AbsoluteReference(targetRef)
	if err != nil {
		return false, err
	}
	featureRef, err = r.r.AbsoluteReference(featureRef)
	if err != nil {
		return false, err
	}

	slog.Debug(fmt.Sprintf("Inspecting gittuf policies to identify if '%s' can be merged into '%s' with current approvals...", featureRef, targetRef))
	verifier := policy.NewPolicyVerifier(r.r)

	var needRSLSignature bool

	if options.BypassRSLForFeatureRef {
		slog.Debug("Not using RSL for feature ref...")
		featureID, err := r.r.GetReference(featureRef)
		if err != nil {
			return false, err
		}

		needRSLSignature, err = verifier.VerifyMergeableForCommit(ctx, targetRef, featureID)
		if err != nil {
			return false, err
		}
	} else {
		needRSLSignature, err = verifier.VerifyMergeable(ctx, targetRef, featureRef)
		if err != nil {
			return false, err
		}
	}

	if needRSLSignature {
		slog.Debug("Merge is allowed but must be performed by authorized user who has not already issued an approval!")
	} else {
		slog.Debug("Merge is allowed and can be performed by any user!")
	}

	return needRSLSignature, nil
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
