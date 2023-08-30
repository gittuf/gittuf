package repository

import (
	"context"

	"github.com/gittuf/gittuf/internal/policy"
)

func (r *Repository) VerifyRef(ctx context.Context, target string, full bool) error {
	target, err := absoluteReference(r.r, target)
	if err != nil {
		return err
	}

	if full {
		return policy.VerifyRefFull(ctx, r.r, target)
	}

	return policy.VerifyRef(ctx, r.r, target)
}
