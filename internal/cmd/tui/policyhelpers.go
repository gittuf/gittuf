// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"context"
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
)

type rule struct {
	name      string
	pattern   string
	key       string
	threshold int
}

type principal struct {
	id             string
	keys           string
	customMetadata string
}

// repoAddPrincipalToTargets adds a person principal to the targets policy file.
func repoAddPrincipalToTargets(ctx context.Context, o *options, personID string, publicKeyRefs []string, associatedIdentities []string, customMetadata []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	// Build the public keys map — same pattern as addperson.go
	publicKeys := map[string]*tufv02.Key{}
	for _, KeyRef := range publicKeyRefs {
		key, err := gittuf.LoadPublicKey(KeyRef)
		if err != nil {
			return err
		}
		publicKeys[key.ID()] = key.(*tufv02.Key)
	}

	// Build the associated identities map — split on "::"
	identities := map[string]string{}
	for _, identity := range associatedIdentities {
		split := strings.Split(identity, "::")
		if len(split) != 2 {
			return fmt.Errorf("invalid format for associated identity '%s', expected 'providerID::identity'", identity)
		}
		identities[split[0]] = split[1]
	}

	// Build the custom metadata map — split on "="
	custom := map[string]string{}
	for _, entry := range customMetadata {
		split := strings.Split(entry, "=")
		if len(split) != 2 {
			return fmt.Errorf("invalid format for associated identity '%s', expected 'KEY=VALUE", entry)
		}
		custom[split[0]] = split[1]
	}

	// Construct the person — same pattern as addperson.go
	person := &tufv02.Person{
		PersonID:             personID,
		PublicKeys:           publicKeys,
		AssociatedIdentities: identities,
		Custom:               custom,
	}

	// Build opts — same pattern as addperson.go
	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.AddPrincipalToTargets(ctx, signer, o.policyName, []tuf.Principal{person}, true, opts...)
}

// getCurrPrincipals returns the current targets principals from the policy file.
func getCurrPrincipals(ctx context.Context, o *options) []principal {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return nil
	}

	principalsMap, err := repo.ListPrincipals(ctx, o.targetRef, o.policyName)
	if err != nil {
		return nil
	}

	currPrincipals := make([]principal, 0, len(principalsMap))
	for _, p := range principalsMap {
		keyIDs := make([]string, 0, len(p.Keys()))
		for _, k := range p.Keys() {
			keyIDs = append(keyIDs, k.KeyID)
		}
		currPrincipals = append(currPrincipals, principal{
			id:             p.ID(),
			keys:           strings.Join(keyIDs, ", "),
			customMetadata: fmt.Sprintf("%v", p.CustomMetadata()),
		})
	}

	return currPrincipals
}

// repoRemovePrincipalFromTargets removes a principal from the targets policy file.
func repoRemovePrincipalFromTargets(ctx context.Context, o *options, principalID string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}

	return repo.RemovePrincipalFromTargets(ctx, signer, o.policyName, principalID, true, opts...)
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
