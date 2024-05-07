// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"errors"

	"github.com/gittuf/gittuf/internal/tuf"
)

var ErrNotCommitOrTag = errors.New("invalid object type, expected commit or tag for signature verification")

func (r *Repository) VerifySignature(ctx context.Context, objectID Hash, key *tuf.Key) error {
	if err := r.ensureIsCommit(objectID); err == nil {
		return r.verifyCommitSignature(ctx, objectID, key)
	}

	if err := r.ensureIsTag(objectID); err == nil {
		return r.verifyTagSignature(ctx, objectID, key)
	}

	return ErrNotCommitOrTag
}
