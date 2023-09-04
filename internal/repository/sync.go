package repository

import (
	"context"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/config"
)

func (r *Repository) Push(ctx context.Context, remoteName string, refNames ...string) error {
	refs := make([]config.RefSpec, 0, len(refNames)+1) // The +1 is for the RSL ref

	for _, refName := range refNames {
		absRefName, err := absoluteReference(r.r, refName)
		if err != nil {
			return err
		}
		refs = append(refs, refSpec(absRefName, false))
	}

	refs = append(refs, refSpec(rsl.RSLRef, false)) // we always want to sync the RSL ref

	return gitinterface.PushRefSpec(ctx, r.r, remoteName, refs)
}

func Clone(ctx context.Context, remoteURL, dir, defaultRef string) error {
	_, err := gitinterface.CloneAndFetch(ctx, remoteURL, dir, defaultRef, []string{rsl.RSLRef, policy.PolicyRef}, false)
	return err
}
