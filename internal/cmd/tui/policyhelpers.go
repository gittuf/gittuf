// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
)

type rule struct {
	name    string
	pattern string
	key     string
}

// getCurrRules returns the current rules from the policy file
func getCurrRules(o *options) []rule {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	rules, err := repo.ListRules(context.Background(), o.targetRef)
	if err != nil {
		return nil
	}

	var currRules = make([]rule, len(rules))
	for i, r := range rules {
		currRules[i] = rule{
			name:    r.Delegation.ID(),
			pattern: strings.Join(r.Delegation.GetProtectedNamespaces(), ", "),
			key:     strings.Join(r.Delegation.GetPrincipalIDs().Contents(), ", "),
		}
	}
	return currRules
}

// repoAddRule adds a rule to the policy file
func repoAddRule(o *options, rule rule, keyPath []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	authorizedPrincipalIDs := []string{}
	for _, key := range keyPath {
		key, err := gittuf.LoadPublicKey(key)
		if err != nil {
			return err
		}

		authorizedPrincipalIDs = append(authorizedPrincipalIDs, key.ID())
	}
	res := repo.AddDelegation(context.Background(), signer, o.policyName, rule.name, authorizedPrincipalIDs, []string{rule.pattern}, 1, true)

	return res
}

// repoRemoveRule removes a rule from the policy file
func repoRemoveRule(o *options, rule rule) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}
	return repo.RemoveDelegation(context.Background(), signer, o.policyName, rule.name, true)
}

// repoReorderRules reorders the rules in the policy file
func repoReorderRules(o *options, rules []rule) error {
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

	return repo.ReorderDelegations(context.Background(), signer, o.policyName, ruleNames, true)
}
