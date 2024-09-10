// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/tuf"
)

const AllowRuleName = "gittuf-allow-rule"

var (
	ErrCannotManipulateAllowRule = errors.New("cannot change in-built gittuf-allow-rule")
	ErrRuleNotFound              = errors.New("cannot find rule entry")
	ErrMissingRules              = errors.New("some rules are missing")
)

// InitializeTargetsMetadata creates a new instance of TargetsMetadata.
func InitializeTargetsMetadata() *tuf.TargetsMetadata {
	targetsMetadata := tuf.NewTargetsMetadata()
	targetsMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	targetsMetadata.Delegations.AddDelegation(AllowRule())
	return targetsMetadata
}

// AddDelegation adds a new delegation to TargetsMetadata.
func AddDelegation(targetsMetadata *tuf.TargetsMetadata, ruleName string, authorizedKeys []*tuf.Key, rulePatterns []string, threshold int) (*tuf.TargetsMetadata, error) {
	if ruleName == AllowRuleName {
		return nil, ErrCannotManipulateAllowRule
	}

	authorizedKeyIDs := []string{}
	for _, key := range authorizedKeys {
		targetsMetadata.Delegations.AddKey(key)

		authorizedKeyIDs = append(authorizedKeyIDs, key.KeyID)
	}

	allDelegations := targetsMetadata.Delegations.Roles
	newDelegation := tuf.Delegation{
		Name:        ruleName,
		Paths:       rulePatterns,
		Terminating: false,
		Role: tuf.Role{
			KeyIDs:    authorizedKeyIDs,
			Threshold: threshold,
		},
	}
	allDelegations = append(allDelegations[:len(allDelegations)-1], newDelegation, AllowRule())

	targetsMetadata.Delegations.Roles = allDelegations

	return targetsMetadata, nil
}

// UpdateDelegation is used to amend a delegation in TargetsMetadata.
func UpdateDelegation(targetsMetadata *tuf.TargetsMetadata, ruleName string, authorizedKeys []*tuf.Key, rulePatterns []string, threshold int) (*tuf.TargetsMetadata, error) {
	if ruleName == AllowRuleName {
		return nil, ErrCannotManipulateAllowRule
	}

	if len(authorizedKeys) < threshold {
		return nil, ErrCannotMeetThreshold
	}

	authorizedKeyIDs := []string{}
	for _, key := range authorizedKeys {
		targetsMetadata.Delegations.AddKey(key)

		authorizedKeyIDs = append(authorizedKeyIDs, key.KeyID)
	}

	allDelegations := []tuf.Delegation{}
	for _, delegation := range targetsMetadata.Delegations.Roles {
		if delegation.Name == AllowRuleName {
			break
		}

		if delegation.Name != ruleName {
			allDelegations = append(allDelegations, delegation)
			continue
		}

		if delegation.Name == ruleName {
			delegation.Paths = rulePatterns
			delegation.Role = tuf.Role{
				KeyIDs:    authorizedKeyIDs,
				Threshold: threshold,
			}
		}

		allDelegations = append(allDelegations, delegation)
	}
	allDelegations = append(allDelegations, AllowRule())

	targetsMetadata.Delegations.Roles = allDelegations

	return targetsMetadata, nil
}

// ReorderDelegations changes the order of delegations, and the new order is
// specified in `ruleNames []string`.
func ReorderDelegations(targetsMetadata *tuf.TargetsMetadata, ruleNames []string) (*tuf.TargetsMetadata, error) {
	// Create a map of all existing delegations for quick look up
	rolesMap := make(map[string]tuf.Delegation)
	// Create a set of current rules in metadata, skipping the allow rule
	currentRules := set.NewSet[string]()
	for _, delegation := range targetsMetadata.Delegations.Roles {
		if delegation.Name == AllowRuleName {
			continue
		}
		rolesMap[delegation.Name] = delegation
		currentRules.Add(delegation.Name)
	}

	specifiedRules := set.NewSet[string]()
	for _, name := range ruleNames {
		if specifiedRules.Has(name) {
			return nil, fmt.Errorf("%w: '%s'", ErrDuplicatedRuleName, name)
		}
		specifiedRules.Add(name)
	}

	if !currentRules.Equal(specifiedRules) {
		onlyInSpecifiedRules := specifiedRules.Minus(currentRules)
		if onlyInSpecifiedRules.Len() != 0 {
			if onlyInSpecifiedRules.Has(AllowRuleName) {
				return nil, fmt.Errorf("%w: do not specify allow rule", ErrCannotManipulateAllowRule)
			}

			contents := onlyInSpecifiedRules.Contents()
			return nil, fmt.Errorf("%w: rules '%s' do not exist in current rule file", ErrRuleNotFound, strings.Join(contents, ", "))
		}

		onlyInCurrentRules := currentRules.Minus(specifiedRules)
		if onlyInCurrentRules.Len() != 0 {
			contents := onlyInCurrentRules.Contents()
			return nil, fmt.Errorf("%w: rules '%s' not specified", ErrMissingRules, strings.Join(contents, ", "))
		}
	}

	// Create newDelegations and set it in the targetsMetadata after adding allow rule
	newDelegations := make([]tuf.Delegation, 0, len(rolesMap)+1)
	for _, ruleName := range ruleNames {
		newDelegations = append(newDelegations, rolesMap[ruleName])
	}
	newDelegations = append(newDelegations, AllowRule())
	targetsMetadata.Delegations.Roles = newDelegations

	return targetsMetadata, nil
}

// RemoveDelegation deletes a delegation entry from TargetsMetadata.
func RemoveDelegation(targetsMetadata *tuf.TargetsMetadata, ruleName string) (*tuf.TargetsMetadata, error) {
	if ruleName == AllowRuleName {
		return nil, ErrCannotManipulateAllowRule
	}

	allDelegations := targetsMetadata.Delegations.Roles
	updatedDelegations := []tuf.Delegation{}

	for _, delegation := range allDelegations {
		if delegation.Name != ruleName {
			updatedDelegations = append(updatedDelegations, delegation)
		}
	}
	targetsMetadata.Delegations.Roles = updatedDelegations

	return targetsMetadata, nil
}

// AddKeyToTargets adds public keys to the specified targets metadata.
func AddKeyToTargets(targetsMetadata *tuf.TargetsMetadata, authorizedKeys []*tuf.Key) (*tuf.TargetsMetadata, error) {
	for _, key := range authorizedKeys {
		targetsMetadata.Delegations.AddKey(key)
	}

	return targetsMetadata, nil
}

// AllowRule returns the default, last rule for all policy files.
func AllowRule() tuf.Delegation {
	return tuf.Delegation{
		Name:        AllowRuleName,
		Paths:       []string{"*"},
		Terminating: true,
		Role: tuf.Role{
			KeyIDs:    []string{},
			Threshold: 1,
		},
	}
}
