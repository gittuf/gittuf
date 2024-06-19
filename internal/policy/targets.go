// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"time"

	"github.com/gittuf/gittuf/internal/tuf"
)

const AllowRuleName = "gittuf-allow-rule"

var ErrCannotManipulateAllowRule = errors.New("cannot change in-built gittuf-allow-rule")

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

// ReorderDelegations changes the order of delegations in TargetsMetadata.
func ReorderDelegations(targetsMetadata *tuf.TargetsMetadata, ruleNames []string) (*tuf.TargetsMetadata, error) {
	allDelegations := []tuf.Delegation{}
	for _, ruleName := range ruleNames {
		if ruleName == AllowRuleName {
			return nil, ErrCannotManipulateAllowRule
		}

		for _, delegation := range targetsMetadata.Delegations.Roles {
			if delegation.Name == ruleName {
				allDelegations = append(allDelegations, delegation)
			}
		}
	}

	allDelegations = append(allDelegations, AllowRule())
	targetsMetadata.Delegations.Roles = allDelegations

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
