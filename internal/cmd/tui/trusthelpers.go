// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"

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

// getGlobalRules returns a slice of globalRule for the TUI
func getGlobalRules(o *options) []globalRule {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	rules, err := repo.ListGlobalRules(context.Background(), o.targetRef)
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
func repoAddGlobalRule(o *options, gr globalRule) error {
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
			context.Background(), signer,
			gr.ruleName, gr.rulePatterns,
			gr.threshold, true, opts...,
		)
	case tuf.GlobalRuleBlockForcePushesType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("namespaces not set for global rule type '%s'", tuf.GlobalRuleBlockForcePushesType)
		}
		return repo.AddGlobalRuleBlockForcePushes(
			context.Background(), signer,
			gr.ruleName, gr.rulePatterns,
			true, opts...,
		)
	default:
		return fmt.Errorf("unknown global rule type %q", gr.ruleType)
	}
}

// repoRemoveGlobalRule removes a global rule
func repoRemoveGlobalRule(o *options, gr globalRule) error {
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
		context.Background(), signer, gr.ruleName, true, opts...,
	)
}

// repoUpdateGlobalRule updates a global rule
func repoUpdateGlobalRule(o *options, gr globalRule) error {
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

		return repo.UpdateGlobalRuleThreshold(context.Background(), signer, gr.ruleName, gr.rulePatterns, gr.threshold, true, opts...)

	case tuf.GlobalRuleBlockForcePushesType:
		if len(gr.rulePatterns) == 0 {
			return fmt.Errorf("namespaces not set for global rule type '%s'", tuf.GlobalRuleBlockForcePushesType)
		}

		return repo.UpdateGlobalRuleBlockForcePushes(context.Background(), signer, gr.ruleName, gr.rulePatterns, true, opts...)

	default:
		return tuf.ErrUnknownGlobalRuleType
	}
}
