// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tuf

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danwakefield/fnmatch"
	"github.com/gittuf/gittuf/internal/common/set"
)

const AllowRuleName = "gittuf-allow-rule"

// TargetsMetadata defines the schema of TUF's Targets role.
type TargetsMetadata struct {
	Type        string         `json:"type"`
	Expires     string         `json:"expires"`
	Targets     map[string]any `json:"targets"`
	Delegations *Delegations   `json:"delegations"`
}

// NewTargetsMetadata returns a new instance of TargetsMetadata.
func NewTargetsMetadata() *TargetsMetadata {
	return &TargetsMetadata{
		Type:        "targets",
		Delegations: &Delegations{},
	}
}

// SetExpires sets the expiry date of the TargetsMetadata to the value passed
// in.
func (t *TargetsMetadata) SetExpires(expires string) {
	t.Expires = expires
}

// Validate ensures the instance of TargetsMetadata matches gittuf expectations.
func (t *TargetsMetadata) Validate() error {
	if len(t.Targets) != 0 {
		return ErrTargetsNotEmpty
	}
	return nil
}

// Delegations defines the schema for specifying delegations in TUF's Targets
// metadata.
type Delegations struct {
	Keys  map[string]*Key `json:"keys"`
	Roles []Delegation    `json:"roles"`
}

// AddKey adds a delegations key.
func (d *Delegations) AddKey(key *Key) {
	if d.Keys == nil {
		d.Keys = map[string]*Key{}
	}

	d.Keys[key.KeyID] = key
}

// AddDelegation adds a new delegation.
func (d *Delegations) AddDelegation(delegation Delegation) {
	if d.Roles == nil {
		d.Roles = []Delegation{}
	}

	d.Roles = append(d.Roles, delegation)
}

// Delegation defines the schema for a single delegation entry. It differs from
// the standard TUF schema by allowing a `custom` field to record details
// pertaining to the delegation.
type Delegation struct {
	Name        string           `json:"name"`
	Paths       []string         `json:"paths"`
	Terminating bool             `json:"terminating"`
	Custom      *json.RawMessage `json:"custom,omitempty"`
	Role
}

// Matches checks if any of the delegation's patterns match the target.
func (d *Delegation) Matches(target string) bool {
	for _, pattern := range d.Paths {
		// We validate pattern when it's added to / updated in the metadata
		if matches := fnmatch.Match(pattern, target, 0); matches {
			return true
		}
	}
	return false
}

// AddDelegation adds a new delegation to TargetsMetadata.
func (t *TargetsMetadata) AddDelegation(ruleName string, authorizedKeys []*Key, rulePatterns []string, threshold int) error {
	if ruleName == AllowRuleName {
		return ErrCannotManipulateAllowRule
	}

	authorizedKeyIDs := set.NewSet[string]()
	for _, key := range authorizedKeys {
		t.Delegations.AddKey(key)

		authorizedKeyIDs.Add(key.KeyID)
	}

	allDelegations := t.Delegations.Roles
	newDelegation := Delegation{
		Name:        ruleName,
		Paths:       rulePatterns,
		Terminating: false,
		Role: Role{
			KeyIDs:    authorizedKeyIDs,
			Threshold: threshold,
		},
	}
	allDelegations = append(allDelegations[:len(allDelegations)-1], newDelegation, AllowRule())
	t.Delegations.Roles = allDelegations
	return nil
}

// UpdateDelegation is used to amend a delegation in TargetsMetadata.
func (t *TargetsMetadata) UpdateDelegation(ruleName string, authorizedKeys []*Key, rulePatterns []string, threshold int) error {
	if ruleName == AllowRuleName {
		return ErrCannotManipulateAllowRule
	}

	if len(authorizedKeys) < threshold {
		return ErrCannotMeetThreshold
	}

	authorizedKeyIDs := set.NewSet[string]()
	for _, key := range authorizedKeys {
		t.Delegations.AddKey(key)

		authorizedKeyIDs.Add(key.KeyID)
	}

	allDelegations := []Delegation{}
	for _, delegation := range t.Delegations.Roles {
		if delegation.Name == AllowRuleName {
			break
		}

		if delegation.Name != ruleName {
			allDelegations = append(allDelegations, delegation)
			continue
		}

		if delegation.Name == ruleName {
			delegation.Paths = rulePatterns
			delegation.Role = Role{
				KeyIDs:    authorizedKeyIDs,
				Threshold: threshold,
			}
		}

		allDelegations = append(allDelegations, delegation)
	}
	allDelegations = append(allDelegations, AllowRule())
	t.Delegations.Roles = allDelegations
	return nil
}

// ReorderDelegations changes the order of delegations, and the new order is
// specified in `ruleNames []string`.
func (t *TargetsMetadata) ReorderDelegations(ruleNames []string) error {
	// Create a map of all existing delegations for quick look up
	rolesMap := make(map[string]Delegation)

	// Create a set of current rules in metadata, skipping the allow rule
	currentRules := set.NewSet[string]()
	for _, delegation := range t.Delegations.Roles {
		if delegation.Name == AllowRuleName {
			continue
		}
		rolesMap[delegation.Name] = delegation
		currentRules.Add(delegation.Name)
	}

	specifiedRules := set.NewSet[string]()
	for _, name := range ruleNames {
		if specifiedRules.Has(name) {
			return fmt.Errorf("%w: '%s'", ErrDuplicatedRuleName, name)
		}
		specifiedRules.Add(name)
	}

	if !currentRules.Equal(specifiedRules) {
		onlyInSpecifiedRules := specifiedRules.Minus(currentRules)
		if onlyInSpecifiedRules.Len() != 0 {
			if onlyInSpecifiedRules.Has(AllowRuleName) {
				return fmt.Errorf("%w: do not specify allow rule", ErrCannotManipulateAllowRule)
			}

			contents := onlyInSpecifiedRules.Contents()
			return fmt.Errorf("%w: rules '%s' do not exist in current rule file", ErrRuleNotFound, strings.Join(contents, ", "))
		}

		onlyInCurrentRules := currentRules.Minus(specifiedRules)
		if onlyInCurrentRules.Len() != 0 {
			contents := onlyInCurrentRules.Contents()
			return fmt.Errorf("%w: rules '%s' not specified", ErrMissingRules, strings.Join(contents, ", "))
		}
	}

	// Create newDelegations and set it in the targetsMetadata after adding allow rule
	newDelegations := make([]Delegation, 0, len(rolesMap)+1)
	for _, ruleName := range ruleNames {
		newDelegations = append(newDelegations, rolesMap[ruleName])
	}
	newDelegations = append(newDelegations, AllowRule())
	t.Delegations.Roles = newDelegations
	return nil
}

// RemoveDelegation deletes a delegation entry from TargetsMetadata.
func (t *TargetsMetadata) RemoveDelegation(ruleName string) error {
	if ruleName == AllowRuleName {
		return ErrCannotManipulateAllowRule
	}

	allDelegations := t.Delegations.Roles
	updatedDelegations := []Delegation{}

	for _, delegation := range allDelegations {
		if delegation.Name != ruleName {
			updatedDelegations = append(updatedDelegations, delegation)
		}
	}
	t.Delegations.Roles = updatedDelegations
	return nil
}

// AllowRule returns the default, last rule for all policy files.
func AllowRule() Delegation {
	return Delegation{
		Name:        AllowRuleName,
		Paths:       []string{"*"},
		Terminating: true,
		Role: Role{
			Threshold: 1,
		},
	}
}
