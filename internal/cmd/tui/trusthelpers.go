// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"sort"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/tuf"
)

type globalRule struct {
	ruleName     string
	ruleType     string
	rulePatterns []string
	threshold    int
}

type rootPrincipal struct {
	principalID string
	keyCount    int
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

func getRootPrincipals(ctx context.Context, o *options) []rootPrincipal {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	rootPrincipals, err := repo.ListRootPrincipals(ctx, o.targetRef)
	if err != nil {
		return nil
	}

	ids := make([]string, 0, len(rootPrincipals))
	for id := range rootPrincipals {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	currPrincipals := make([]rootPrincipal, 0, len(rootPrincipals))
	for _, id := range ids {
		currPrincipals = append(currPrincipals, rootPrincipal{
			principalID: id,
			keyCount:    len(rootPrincipals[id].Keys()),
		})
	}
	return currPrincipals
}

func getPrimaryRuleFilePrincipals(ctx context.Context, o *options) []rootPrincipal {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	primaryRuleFilePrincipals, err := repo.ListPrimaryRuleFilePrincipals(ctx, o.targetRef)
	if err != nil {
		return nil
	}

	ids := make([]string, 0, len(primaryRuleFilePrincipals))
	for id := range primaryRuleFilePrincipals {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	currPrincipals := make([]rootPrincipal, 0, len(primaryRuleFilePrincipals))
	for _, id := range ids {
		currPrincipals = append(currPrincipals, rootPrincipal{
			principalID: id,
			keyCount:    len(primaryRuleFilePrincipals[id].Keys()),
		})
	}
	return currPrincipals
}

func repoAddRootPrincipal(ctx context.Context, o *options, role, principalSource string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}
	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	principal, err := gittuf.LoadPublicKey(principalSource)
	if err != nil {
		return err
	}

	var opts []trustpolicyopts.Option
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	switch strings.ToLower(strings.TrimSpace(role)) {
	case "root":
		return repo.AddRootKey(ctx, signer, principal, true, opts...)
	case "policy":
		return repo.AddTopLevelTargetsKey(ctx, signer, principal, true, opts...)
	default:
		return fmt.Errorf("unknown role %q (expected 'root' or 'policy')", role)
	}
}

func repoRemoveRootPrincipal(ctx context.Context, o *options, role, principalID string) error {
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

	id := strings.ToLower(strings.TrimSpace(principalID))
	switch strings.ToLower(strings.TrimSpace(role)) {
	case "root":
		return repo.RemoveRootKey(ctx, signer, id, true, opts...)
	case "policy":
		return repo.RemoveTopLevelTargetsKey(ctx, signer, id, true, opts...)
	default:
		return fmt.Errorf("unknown role %q (expected 'root' or 'policy')", role)
	}
}
