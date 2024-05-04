// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

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
func (r *Repository) PushPolicy(remoteName string) error {
	slog.Debug(fmt.Sprintf("Pushing policy and RSL references to %s...", remoteName))
	if err := r.r.Push(remoteName, []string{policy.PolicyRef, policy.PolicyStagingRef, rsl.Ref}); err != nil {
		return errors.Join(ErrPushingPolicy, err)
	}

	return nil
}

// PullPolicy fetches gittuf policy from the specified remote. The fetches is
// marked as fast forward only to detect divergence. Note that this also fetches
// the RSL as the policy must be updated in sync with the RSL.
func (r *Repository) PullPolicy(remoteName string) error {
	slog.Debug(fmt.Sprintf("Pulling policy and RSL references from %s...", remoteName))
	if err := r.r.Fetch(remoteName, []string{policy.PolicyRef, policy.PolicyStagingRef, rsl.Ref}, true); err != nil {
		return errors.Join(ErrPullingPolicy, err)
	}

	return nil
}

func (r *Repository) ApplyPolicy(ctx context.Context, signRSLEntry bool) error {
	return policy.Apply(ctx, r.r, signRSLEntry)
}

func (r *Repository) ListRules(ctx context.Context, targetRef string) ([]*policy.DelegationWithDepth, error) {
	if strings.HasPrefix(targetRef, "refs/gittuf/") {
		return policy.ListRules(ctx, r.r, targetRef)
	}
	return policy.ListRules(ctx, r.r, "refs/gittuf/"+targetRef)
}
