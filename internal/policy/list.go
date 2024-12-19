// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/tuf"
)

type DelegationWithDepth struct {
	Delegation tuf.Rule
	Depth      int
}

// ListRules returns a list of all the rules as an array of the delegations in a
// pre-order traversal of the delegation tree, with the depth of each
// delegation.
func ListRules(ctx context.Context, repo *gitinterface.Repository, targetRef string) ([]*DelegationWithDepth, error) {
	state, err := LoadCurrentState(ctx, repo, targetRef)
	if err != nil {
		return nil, err
	}

	if !state.HasTargetsRole(TargetsRoleName) {
		return nil, nil
	}

	topLevelTargetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, true)
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

	seenRoles := map[string]bool{TargetsRoleName: true}

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

// ListPrincipals returns the principals present in the specified rule file.
// `targetRef` can be used to control which policy reference is used.
func ListPrincipals(ctx context.Context, repo *gitinterface.Repository, targetRef, policyName string) (map[string]tuf.Principal, error) {
	state, err := LoadCurrentState(ctx, repo, targetRef)
	if err != nil {
		return nil, err
	}

	if !state.HasTargetsRole(policyName) {
		return nil, ErrPolicyNotFound
	}

	metadata, err := state.GetTargetsMetadata(policyName, false)
	if err != nil {
		return nil, err
	}

	return metadata.GetPrincipals(), nil
}

// ListHooks returns the hooks present in the specified rule file.
// `targetRef` can be used to control which policy reference is used.
func ListHooks(ctx context.Context, repo *gitinterface.Repository, targetRef, policyName string) (map[string]map[string]tuf.Applet, error) {
	state, err := LoadCurrentState(ctx, repo, targetRef)
	if err != nil {
		return nil, err
	}

	if !state.HasTargetsRole(policyName) {
		return nil, ErrPolicyNotFound
	}

	metadata, err := state.GetTargetsMetadata(policyName, false)
	if err != nil {
		return nil, err
	}

	var hooks map[string]map[string]tuf.Applet

	// pre-commit
	preCommitHooks, err := metadata.GetHooks("pre-commit")
	if err != nil {
		return nil, err
	}

	for name, hook := range preCommitHooks {
		hooks["pre-commit"][name] = hook
	}

	// pre-push
	prePushHooks, err := metadata.GetHooks("pre-push")
	if err != nil {
		return nil, err
	}

	for name, hook := range prePushHooks {
		hooks["pre-push"][name] = hook
	}

	return hooks, nil
}
