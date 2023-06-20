package policy

import (
	"errors"
	"time"

	"github.com/adityasaky/gittuf/internal/tuf"
)

const AllowRuleName = "gittuf-allow-rule"

var ErrCannotManipulateAllowRule = errors.New("cannot change in-built gittuf-allow-rule")

// InitializeTargetsMetadata creates a new instance of TargetsMetadata.
func InitializeTargetsMetadata() *tuf.TargetsMetadata {
	targetsMetadata := tuf.NewTargetsMetadata()
	targetsMetadata.SetVersion(1)
	targetsMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	targetsMetadata.Delegations.AddDelegation(AllowRule())
	return targetsMetadata
}

// AddOrUpdateDelegation is used to add or amend a delegation in
// TargetsMetadata.
func AddOrUpdateDelegation(targetsMetadata *tuf.TargetsMetadata, ruleName string, authorizedKeys []*tuf.Key, rulePatterns []string) (*tuf.TargetsMetadata, error) {
	if ruleName == AllowRuleName {
		return nil, ErrCannotManipulateAllowRule
	}

	authorizedKeyIDs := []string{}
	for _, key := range authorizedKeys {
		targetsMetadata.Delegations.AddKey(key)

		authorizedKeyIDs = append(authorizedKeyIDs, key.KeyID)
	}

	allDelegations := []tuf.Delegation{}

	existingDelegation := false
	for _, delegation := range targetsMetadata.Delegations.Roles {
		if delegation.Name == AllowRuleName {
			break
		}

		if delegation.Name == ruleName {
			// update existing delegation
			existingDelegation = true
			delegation.Paths = rulePatterns
			delegation.Role = tuf.Role{KeyIDs: authorizedKeyIDs, Threshold: 1}
		}

		allDelegations = append(allDelegations, delegation)
	}

	if !existingDelegation {
		allDelegations = append(allDelegations, tuf.Delegation{
			Name:        ruleName,
			Paths:       rulePatterns,
			Terminating: false,
			Role: tuf.Role{
				KeyIDs:    authorizedKeyIDs,
				Threshold: 1,
			},
		})
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
