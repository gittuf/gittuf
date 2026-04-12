// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/tui/trust"
	"github.com/gittuf/gittuf/internal/policy"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/internal/tuf"
)

type globalRule struct {
	ruleName     string
	ruleType     string
	rulePatterns []string
	threshold    int
}

// getGlobalRules returns a slice of globalRule for the TUI
func getGlobalRules(ctx context.Context, o *options) []globalRule {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	rules, err := repo.ListGlobalRules(ctx, o.targetRef)
	if err != nil {
		return nil
	}

	var currRules = make([]globalRule, len(rules))
	for i, r := range rules {
		switch gRule := r.(type) {
		case tuf.GlobalRuleThreshold:
			currRules[i] = globalRule{
				ruleName:     gRule.GetName(),
				ruleType:     tuf.GlobalRuleThresholdType,
				rulePatterns: gRule.GetProtectedNamespaces(),
				threshold:    gRule.GetThreshold(),
			}
		case tuf.GlobalRuleBlockForcePushes:
			currRules[i] = globalRule{
				ruleName:     gRule.GetName(),
				ruleType:     tuf.GlobalRuleBlockForcePushesType,
				rulePatterns: gRule.GetProtectedNamespaces(),
			}
		}
	}
	return currRules
}

// repoAddGlobalRule adds a global rule
func repoAddGlobalRule(ctx context.Context, o *options, gr globalRule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	switch gr.ruleType {
	case tuf.GlobalRuleThresholdType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("namespaces not set for global rule type '%s'", tuf.GlobalRuleThresholdType)
		}
		return repo.AddGlobalRuleThreshold(
			ctx, signer,
			gr.ruleName, gr.rulePatterns,
			gr.threshold, true, opts...,
		)
	case tuf.GlobalRuleBlockForcePushesType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("namespaces not set for global rule type '%s'", tuf.GlobalRuleBlockForcePushesType)
		}
		return repo.AddGlobalRuleBlockForcePushes(
			ctx, signer,
			gr.ruleName, gr.rulePatterns,
			true, opts...,
		)
	default:
		return fmt.Errorf("unknown global rule type %q", gr.ruleType)
	}
}

// repoRemoveGlobalRule removes a global rule
func repoRemoveGlobalRule(ctx context.Context, o *options, gr globalRule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.RemoveGlobalRule(
		ctx, signer, gr.ruleName, true, opts...,
	)
}

// repoUpdateGlobalRule updates a global rule
func repoUpdateGlobalRule(ctx context.Context, o *options, gr globalRule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	switch gr.ruleType {
	case tuf.GlobalRuleThresholdType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("namespaces not set for global rule type '%s'", tuf.GlobalRuleThresholdType)
		}

		return repo.UpdateGlobalRuleThreshold(ctx, signer, gr.ruleName, gr.rulePatterns, gr.threshold, true, opts...)

	case tuf.GlobalRuleBlockForcePushesType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("namespaces not set for global rule type '%s'", tuf.GlobalRuleBlockForcePushesType)
		}

		return repo.UpdateGlobalRuleBlockForcePushes(ctx, signer, gr.ruleName, gr.rulePatterns, true, opts...)

	default:
		return tuf.ErrUnknownGlobalRuleType
	}
}

// getRootConfig returns the root configuration state for the TUI.
func getRootConfig(ctx context.Context, o *options) (trust.Model, error) {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return trust.Model{}, err
	}

	state, err := policy.LoadCurrentState(ctx, repo.GetGitRepository(), policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return trust.Model{}, err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return trust.Model{}, err
	}

	threshold, err := rootMetadata.GetRootThreshold()
	if err != nil {
		return trust.Model{}, err
	}

	principals, err := rootMetadata.GetRootPrincipals()
	if err != nil {
		return trust.Model{}, err
	}

	principalIDs := make([]string, len(principals))
	for i, principal := range principals {
		principalIDs[i] = principal.ID()
	}

	return trust.InitialModel(rootMetadata.GetRepositoryLocation(), threshold, principalIDs), nil
}

func repoUpdateRootConfig(ctx context.Context, o *options, updated trust.UpdatedTrustMsg, current trust.Model) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	if updated.Threshold != current.OriginalThreshold {
		if err := repo.UpdateRootThreshold(ctx, signer, updated.Threshold, true, opts...); err != nil {
			return err
		}
	}

	if updated.RepoPath != current.OriginalRepoPath {
		if err := repo.SetRepositoryLocation(ctx, signer, updated.RepoPath, true, opts...); err != nil {
			return err
		}
	}

	return nil
}

func repoAddRootKey(ctx context.Context, o *options, keyRef string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	newRootKey, err := gittuf.LoadPublicKey(keyRef)
	if err != nil {
		return err
	}

	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.AddRootKey(ctx, signer, newRootKey, true, opts...)
}

func repoRemoveRootKey(ctx context.Context, o *options, keyID string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.RemoveRootKey(ctx, signer, keyID, true, opts...)
}
