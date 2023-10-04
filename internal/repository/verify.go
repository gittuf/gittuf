package repository

import (
	"context"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
)

func (r *Repository) VerifyRef(ctx context.Context, target string, full bool) error {
	target, err := gitinterface.AbsoluteReference(r.r, target)
	if err != nil {
		return err
	}

	if full {
		return policy.VerifyRefFull(ctx, r.r, target)
	}

	return policy.VerifyRef(ctx, r.r, target)
}

func (r *Repository) VerifyCommit(ctx context.Context, ids ...string) map[string]string {
	return policy.VerifyCommit(ctx, r.r, ids...)
}
