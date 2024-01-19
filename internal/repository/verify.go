// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"

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

func (r *Repository) VerifyRef(ctx context.Context, target string, latestOnly bool, from string) error {
	var (
		expectedTip plumbing.Hash
		err         error
	)

	target, err = gitinterface.AbsoluteReference(r.r, target)
	if err != nil {
		return err
	}

	switch {
	case from != "":
		expectedTip, err = policy.VerifyFromRef(ctx, r.r, target, from)
	case latestOnly:
		expectedTip, err = policy.VerifyRef(ctx, r.r, target)
	default:
		expectedTip, err = policy.VerifyRefFull(ctx, r.r, target)
	}

	if err != nil {
		return err
	}

	return r.verifyRefTip(target, expectedTip)
}

func (r *Repository) VerifyCommit(ctx context.Context, ids ...string) map[string]string {
	return policy.VerifyCommit(ctx, r.r, ids...)
}

func (r *Repository) VerifyTag(ctx context.Context, ids []string) map[string]string {
	return policy.VerifyTag(ctx, r.r, ids)
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
