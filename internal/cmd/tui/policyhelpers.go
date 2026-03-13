// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
)

type rule struct {
	name      string
	pattern   string
	key       string
	threshold int
}

// getCurrRules returns the current rules from the policy file.
func getCurrRules(ctx context.Context, o *options) []rule {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	rules, err := repo.ListRules(ctx, o.targetRef)
	if err != nil {
		return nil
	}

	var currRules = make([]rule, len(rules))
	for i, r := range rules {
		currRules[i] = rule{
			name:      r.Delegation.ID(),
			pattern:   strings.Join(r.Delegation.GetProtectedNamespaces(), ", "),
			key:       strings.Join(r.Delegation.GetPrincipalIDs().Contents(), ", "),
			threshold: r.Delegation.GetThreshold(),
		}
	}
	return currRules
}

// repoAddRule adds a rule to the policy file.
func repoAddRule(ctx context.Context, o *options, rule rule, authorizedPrincipalIDs []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.AddDelegation(ctx, signer, o.policyName, rule.name, authorizedPrincipalIDs, []string{rule.pattern}, rule.threshold, true)
}

// repoUpdateRule updates an existing rule in the policy file.
func repoUpdateRule(ctx context.Context, o *options, r rule, authorizedPrincipalIDs []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.UpdateDelegation(ctx, signer, o.policyName, r.name, authorizedPrincipalIDs, []string{r.pattern}, r.threshold, true)
}

// repoRemoveRule removes a rule from the policy file.
func repoRemoveRule(ctx context.Context, o *options, rule rule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	return repo.RemoveDelegation(ctx, signer, o.policyName, rule.name, true)
}

// repoReorderRules reorders the rules in the policy file.
func repoReorderRules(ctx context.Context, o *options, rules []rule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	ruleNames := make([]string, len(rules))
	for i, r := range rules {
		ruleNames[i] = r.name
	}

	return repo.ReorderDelegations(ctx, signer, o.policyName, ruleNames, true)
}
