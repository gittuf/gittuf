// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/tuf"
)

var (
	ErrPushingPolicy     = errors.New("unable to push policy")
	ErrPullingPolicy     = errors.New("unable to pull policy")
	ErrNoRemoteSpecified = errors.New("no remote specified to push policy")
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

func (r *Repository) ApplyPolicy(ctx context.Context, remoteName string, localOnly, signRSLEntry bool) error {
	if signRSLEntry {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	if remoteName == "" && !localOnly {
		return ErrNoRemoteSpecified
	}

	if !localOnly {
		_, err := r.Sync(ctx, remoteName, false, signRSLEntry)
		if err != nil {
			return err
		}
	}

	if err := policy.Apply(ctx, r.r, signRSLEntry); err != nil {
		return err
	}

	if err := r.RecordRSLEntryForReference(ctx, policy.PolicyRef, signRSLEntry, rslopts.WithRecordLocalOnly()); err != nil {
		return err
	}

	if localOnly {
		return nil
	}

	_, err := r.Sync(ctx, remoteName, false, signRSLEntry)
	return err
}

func (r *Repository) DiscardPolicy() error {
	return policy.Discard(r.r)
}

func (r *Repository) ListRules(ctx context.Context, targetRef string) ([]*policy.DelegationWithDepth, error) {
	if strings.HasPrefix(targetRef, "refs/gittuf/") {
		return policy.ListRules(ctx, r.r, targetRef)
	}
	return policy.ListRules(ctx, r.r, "refs/gittuf/"+targetRef)
}

func (r *Repository) ListPrincipals(ctx context.Context, targetRef, policyName string) (map[string]tuf.Principal, error) {
	if strings.HasPrefix(targetRef, "refs/gittuf/") {
		return policy.ListPrincipals(ctx, r.r, targetRef, policyName)
	}
	return policy.ListPrincipals(ctx, r.r, "refs/gittuf/"+targetRef, policyName)
}

func (r *Repository) StagePolicy(ctx context.Context, remoteName string, localOnly, signCommit bool) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	opts := []rslopts.RecordOption{rslopts.WithRecordRemote(remoteName)}
	if localOnly {
		opts = append(opts, rslopts.WithRecordLocalOnly())
	}

	return r.RecordRSLEntryForReference(ctx, policy.PolicyStagingRef, signCommit, opts...)
}
