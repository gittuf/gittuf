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
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
)

// StageAllSentinel is the single-element value of selectedTargets that means
// "promote the entire policy-index tip into policy-staging". Selective stages
// are expressed by passing the explicit target names. An empty slice is an
// error — callers must opt in to staging everything via this sentinel.
const StageAllSentinel = "."

var (
	ErrPushingPolicy     = errors.New("unable to push policy")
	ErrPullingPolicy     = errors.New("unable to pull policy")
	ErrNoRemoteSpecified = errors.New("no remote specified to push policy")
	ErrNoTargetsSelected = errors.New("no policy targets selected; pass target name(s) or the stage-all sentinel")
)

// PushPolicy pushes the local gittuf policy to the specified remote. As this
// push defaults to fast-forward only, divergent policy states are detected.
// Note that this also pushes the RSL as the policy cannot change without an
// update to the RSL. PolicyIndexRef is intentionally excluded — it is the
// local scratchpad of pending mutations and is not meant to be shared;
// co-maintainers interact with PolicyStagingRef (the official proposal) instead.
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
// PolicyIndexRef is intentionally excluded — see PushPolicy.
func (r *Repository) PullPolicy(remoteName string) error {
	slog.Debug(fmt.Sprintf("Pulling policy and RSL references from %s...", remoteName))
	if err := r.r.Fetch(remoteName, []string{policy.PolicyRef, policy.PolicyStagingRef, rsl.Ref}, true); err != nil {
		return errors.Join(ErrPullingPolicy, err)
	}

	return nil
}

// HasPolicy indicates if the repository has a gittuf policy applied.
func (r *Repository) HasPolicy() (bool, error) {
	_, _, err := rsl.GetLatestReferenceUpdaterEntry(r.r, rsl.ForReference(policy.PolicyRef), rsl.IsUnskipped())
	if err != nil {
		if errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return false, nil
		}

		return false, err
	}

	return true, nil
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

// DiscardPolicy resets both PolicyStagingRef and PolicyIndexRef to PolicyRef,
// dropping any in-flight policy changes.
func (r *Repository) DiscardPolicy() error {
	return policy.Discard(r.r)
}

type DelegationWithDepth struct {
	Delegation tuf.Rule
	Depth      int
}

func (r *Repository) ListRules(ctx context.Context, targetRef string) ([]*DelegationWithDepth, error) {
	if !strings.HasPrefix(targetRef, "refs/gittuf/") {
		targetRef = "refs/gittuf/" + targetRef
	}

	state, err := policy.LoadCurrentState(ctx, r.r, targetRef)
	if err != nil {
		return nil, err
	}

	if !state.HasTargetsRole(policy.TargetsRoleName) {
		return nil, nil
	}

	topLevelTargetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, true)
	if err != nil {
		return nil, err
	}

	delegationsToSearch := []*DelegationWithDepth{}
	allDelegations := []*DelegationWithDepth{}

	for _, topLevelDelegation := range topLevelTargetsMetadata.GetRules() {
		if topLevelDelegation.ID() == tuf.AllowRuleName {
			continue
		}
		delegationsToSearch = append(delegationsToSearch, &DelegationWithDepth{Delegation: topLevelDelegation, Depth: 0})
	}

	seenRoles := map[string]bool{policy.TargetsRoleName: true}

	for len(delegationsToSearch) > 0 {
		currentDelegation := delegationsToSearch[0]
		delegationsToSearch = delegationsToSearch[1:]

		// allDelegations will be the returned list of all the delegations in pre-order traversal, no delegations will be popped off
		allDelegations = append(allDelegations, currentDelegation)

		if _, seen := seenRoles[currentDelegation.Delegation.ID()]; seen {
			continue
		}

		if state.HasTargetsRole(currentDelegation.Delegation.ID()) {
			currentMetadata, err := state.GetTargetsMetadata(currentDelegation.Delegation.ID(), true)
			if err != nil {
				return nil, err
			}

			seenRoles[currentDelegation.Delegation.ID()] = true

			// We construct localDelegations first so that we preserve the order
			// of delegations in currentMetadata in delegationsToSearch
			localDelegations := []*DelegationWithDepth{}
			for _, delegation := range currentMetadata.GetRules() {
				if delegation.ID() == tuf.AllowRuleName {
					continue
				}
				localDelegations = append(localDelegations, &DelegationWithDepth{Delegation: delegation, Depth: currentDelegation.Depth + 1})
			}

			if len(localDelegations) > 0 {
				delegationsToSearch = append(localDelegations, delegationsToSearch...)
			}
		}
	}

	return allDelegations, nil
}

func (r *Repository) ListPrincipals(ctx context.Context, targetRef, policyName string) (map[string]tuf.Principal, error) {
	if !strings.HasPrefix(targetRef, "refs/gittuf/") {
		targetRef = "refs/gittuf/" + targetRef
	}

	state, err := policy.LoadCurrentState(ctx, r.r, targetRef)
	if err != nil {
		return nil, err
	}

	if !state.HasTargetsRole(policyName) {
		return nil, policy.ErrPolicyNotFound
	}

	metadata, err := state.GetTargetsMetadata(policyName, false)
	if err != nil {
		return nil, err
	}

	return metadata.GetPrincipals(), nil
}

// ListGlobalRules returns a list of all global rules as an array of tuf.GlobalRules.
func (r *Repository) ListGlobalRules(ctx context.Context, targetRef string) ([]tuf.GlobalRule, error) {
	if !strings.HasPrefix(targetRef, "refs/gittuf/") {
		targetRef = "refs/gittuf/" + targetRef
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, targetRef)
	if err != nil {
		return nil, err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return nil, err
	}

	return rootMetadata.GetGlobalRules(), nil
}

func (r *Repository) ListHooks(ctx context.Context, targetRef string) (map[tuf.HookStage][]tuf.Hook, error) {
	if !strings.HasPrefix(targetRef, "refs/gittuf/") {
		targetRef = "refs/gittuf/" + targetRef
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, targetRef)
	if err != nil {
		return nil, err
	}

	return state.Hooks, nil
}

// StagePolicy promotes pending policy changes from PolicyIndexRef into
// PolicyStagingRef (the officially proposed policy state). Pass selectedTargets
// as []string{StageAllSentinel} to copy the entire PolicyIndexRef tip into
// PolicyStagingRef; pass one or more explicit target names to overlay only
// those envelopes onto the current staged state (or PolicyRef if no staged
// ref exists yet). A structural coupling check is performed on the selective
// path — signatures are not verified here; that happens at apply time. An
// empty selectedTargets returns ErrNoTargetsSelected.
func (r *Repository) StagePolicy(ctx context.Context, remoteName string, selectedTargets []string, localOnly, signCommit bool) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		if err := r.r.CanSign(); err != nil {
			return err
		}
	}

	if remoteName == "" && !localOnly {
		return ErrNoRemoteSpecified
	}

	if len(selectedTargets) == 0 {
		return ErrNoTargetsSelected
	}

	stageAll := len(selectedTargets) == 1 && selectedTargets[0] == StageAllSentinel

	// Sync from remote first so we're staging on top of the latest state.
	if !localOnly {
		if _, err := r.Sync(ctx, remoteName, false, signCommit); err != nil {
			return err
		}
	}

	if stageAll {
		// Default behavior: fully promote PolicyIndexRef tip into PolicyStagingRef.
		indexTip, err := r.r.GetReference(policy.PolicyIndexRef)
		if err != nil {
			return fmt.Errorf("failed to get policy index reference: %w", err)
		}
		if err := r.r.SetReference(policy.PolicyStagingRef, indexTip); err != nil {
			return fmt.Errorf("failed to set policy staging reference: %w", err)
		}
	} else {
		// Selective: overlay only the named target envelopes from PolicyIndexRef
		// onto the current staged proposal (falling back to PolicyRef if no
		// staged proposal exists yet, or an empty base on initial bootstrap
		// when neither exists).
		baseState, err := loadStagingBaseState(ctx, r.r)
		if err != nil {
			return err
		}

		sourceState, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyIndexRef, policyopts.BypassRSL())
		if err != nil {
			return fmt.Errorf("failed to load policy-index state: %w", err)
		}

		// The root envelope is the trust anchor for every other envelope, so
		// always promote it from the index regardless of --policy-name. Without
		// it, the staged proposal can't be verified at apply time.
		overlaySelectors := selectedTargets
		if sourceState.Metadata != nil && sourceState.Metadata.RootEnvelope != nil {
			hasRoot := false
			for _, name := range overlaySelectors {
				if name == policy.RootRoleName {
					hasRoot = true
					break
				}
			}
			if !hasRoot {
				overlaySelectors = append([]string{policy.RootRoleName}, overlaySelectors...)
			}
		}

		overlayState, err := policy.BuildOverlayState(ctx, baseState, sourceState, overlaySelectors)
		if err != nil {
			return err
		}

		message := fmt.Sprintf("Stage: %s\n", strings.Join(selectedTargets, ","))
		if _, err := policy.StageOverlayCommit(r.r, overlayState, message, signCommit); err != nil {
			return err
		}
	}

	// Record RSL entry locally for the staged proposal (idempotent if
	// StageOverlayCommit already did so on the selective path).
	// PolicyIndexRef is intentionally not recorded in the RSL — it is local-only
	// and the RSL is a shared, push-able log.
	if err := r.RecordRSLEntryForReference(ctx, policy.PolicyStagingRef, signCommit, rslopts.WithRecordLocalOnly()); err != nil {
		return err
	}

	if localOnly {
		return nil
	}

	// Push the updated PolicyStagingRef and RSL.
	_, err := r.Sync(ctx, remoteName, false, signCommit)
	return err
}

// loadStagingBaseState returns the *policy.State that selective stage should
// overlay onto. Preference order:
//   1. PolicyStagingRef (the current staged proposal — most common).
//   2. PolicyRef (no proposal in flight, base on the applied policy).
//   3. An empty State (initial bootstrap — neither applied nor proposed yet).
//
// Falling through to an empty base lets a user run `gittuf policy stage
// --policy-name X` in a fresh repo without first having to do a full stage.
// The overlay then carries just the selected envelopes; verification at apply
// time will catch any missing pieces (e.g., absent root/targets envelopes).
func loadStagingBaseState(ctx context.Context, repo *gitinterface.Repository) (*policy.State, error) {
	if _, err := repo.GetReference(policy.PolicyStagingRef); err == nil {
		return policy.LoadCurrentState(ctx, repo, policy.PolicyStagingRef, policyopts.BypassRSL())
	} else if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
		return nil, fmt.Errorf("failed to get policy staging reference: %w", err)
	}

	if _, err := repo.GetReference(policy.PolicyRef); err == nil {
		return policy.LoadCurrentState(ctx, repo, policy.PolicyRef, policyopts.BypassRSL())
	} else if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
		return nil, fmt.Errorf("failed to get policy reference: %w", err)
	}

	// Initial bootstrap — neither ref exists. Return an empty state so the
	// overlay just becomes the named envelopes from the index.
	return &policy.State{Metadata: &policy.StateMetadata{}}, nil
}

