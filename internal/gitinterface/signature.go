// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"errors"

	"github.com/gittuf/gittuf/internal/tuf"
)

var ErrNotCommitOrTag = errors.New("invalid object type, expected commit or tag for signature verification")

// VerifySignature verifies the cryptographic signature associated with the
// specified object. The `objectID` must point to a Git commit or tag object.
func (r *Repository) VerifySignature(ctx context.Context, objectID Hash, key *tuf.Key) error {
	if err := r.ensureIsCommit(objectID); err == nil {
		return r.verifyCommitSignature(ctx, objectID, key)
	}

	if err := r.ensureIsTag(objectID); err == nil {
		return r.verifyTagSignature(ctx, objectID, key)
	}

	return ErrNotCommitOrTag
}
