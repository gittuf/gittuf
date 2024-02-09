// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
)

var (
	ErrPushingPolicy = errors.New("unable to push policy")
	ErrPullingPolicy = errors.New("unable to pull policy")
)

// PushPolicy pushes the local gittuf policy to the specified remote. As this
// push defaults to fast-forward only, divergent policy states are detected.
// Note that this also pushes the RSL as the policy cannot change without an
// update to the RSL.
func (r *Repository) PushPolicy(ctx context.Context, remoteName string) error {
	slog.Debug(fmt.Sprintf("Pushing policy and RSL references to %s...", remoteName))
	if err := gitinterface.Push(ctx, r.r, remoteName, []string{policy.PolicyRef, rsl.Ref}); err != nil {
		return errors.Join(ErrPushingPolicy, err)
	}

	return nil
}

// PullPolicy fetches gittuf policy from the specified remote. The fetches is
// marked as fast forward only to detect divergence. Note that this also fetches
// the RSL as the policy must be updated in sync with the RSL.
func (r *Repository) PullPolicy(ctx context.Context, remoteName string) error {
	slog.Debug(fmt.Sprintf("Pulling policy and RSL references from %s...", remoteName))
	if err := gitinterface.Fetch(ctx, r.r, remoteName, []string{policy.PolicyRef, rsl.Ref}, true); err != nil {
		return errors.Join(ErrPullingPolicy, err)
	}

	return nil
}

func (r *Repository) ListRules(ctx context.Context) ([]*policy.DelegationWithDepth, error) {
	return policy.ListRules(ctx, r.r)
}
