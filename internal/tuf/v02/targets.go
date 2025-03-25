// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/danwakefield/fnmatch"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/tuf"
)

const (
	TargetsVersion = "http://gittuf.dev/policy/rule-file/v0.2"
)

var ErrTargetsNotEmpty = errors.New("`targets` field in gittuf Targets metadata must be empty")

// TargetsMetadata defines the schema of TUF's Targets role.
type TargetsMetadata struct {
	Type        string         `json:"type"`
	Version     string         `json:"schemaVersion"`
	Expires     string         `json:"expires"`
	Targets     map[string]any `json:"targets"`
	Delegations *Delegations   `json:"delegations"`
}

// NewTargetsMetadata returns a new instance of TargetsMetadata.
func NewTargetsMetadata() *TargetsMetadata {
	return &TargetsMetadata{
		Type:        "targets",
		Version:     TargetsVersion,
		Delegations: &Delegations{Roles: []*Delegation{AllowRule()}},
	}
}

// SetExpires sets the expiry date of the TargetsMetadata to the value passed
// in.
func (t *TargetsMetadata) SetExpires(expires string) {
	t.Expires = expires
}

// SchemaVersion returns the metadata schema version.
func (t *TargetsMetadata) SchemaVersion() string {
	return t.Version
}

// Validate ensures the instance of TargetsMetadata matches gittuf expectations.
func (t *TargetsMetadata) Validate() error {
	if len(t.Targets) != 0 {
		return ErrTargetsNotEmpty
	}
	return nil
}

// AddRule adds a new delegation to TargetsMetadata.
func (t *TargetsMetadata) AddRule(ruleName string, authorizedPrincipalIDs, rulePatterns []string, threshold int) error {
	if strings.HasPrefix(ruleName, tuf.GittufPrefix) {
		return tuf.ErrCannotManipulateRulesWithGittufPrefix
	}

	for _, principalID := range authorizedPrincipalIDs {
		if _, has := t.Delegations.Principals[principalID]; !has {
			return tuf.ErrPrincipalNotFound
		}
	}

	if len(authorizedPrincipalIDs) < threshold {
		return tuf.ErrCannotMeetThreshold
	}

	allDelegations := t.Delegations.Roles
	if allDelegations == nil {
		allDelegations = []*Delegation{}
	}

	newDelegation := &Delegation{
		Name:        ruleName,
		Paths:       rulePatterns,
		Terminating: false,
		Role: Role{
			PrincipalIDs: set.NewSetFromItems(authorizedPrincipalIDs...),
			Threshold:    threshold,
		},
	}
	allDelegations = append(allDelegations[:len(allDelegations)-1], newDelegation, AllowRule())
	t.Delegations.Roles = allDelegations
	return nil
}

// UpdateRule is used to amend a delegation in TargetsMetadata.
func (t *TargetsMetadata) UpdateRule(ruleName string, authorizedPrincipalIDs, rulePatterns []string, threshold int) error {
	if strings.HasPrefix(ruleName, tuf.GittufPrefix) {
		return tuf.ErrCannotManipulateRulesWithGittufPrefix
	}

	for _, principalID := range authorizedPrincipalIDs {
		if _, has := t.Delegations.Principals[principalID]; !has {
			return tuf.ErrPrincipalNotFound
		}
	}

	if len(authorizedPrincipalIDs) < threshold {
		return tuf.ErrCannotMeetThreshold
	}

	allDelegations := []*Delegation{}
	for _, delegation := range t.Delegations.Roles {
		if delegation.ID() == tuf.AllowRuleName {
			break
		}

		if delegation.ID() != ruleName {
			allDelegations = append(allDelegations, delegation)
			continue
		}

		if delegation.Name == ruleName {
			delegation.Paths = rulePatterns
			delegation.Role = Role{
				PrincipalIDs: set.NewSetFromItems(authorizedPrincipalIDs...),
				Threshold:    threshold,
			}
		}

		allDelegations = append(allDelegations, delegation)
	}
	allDelegations = append(allDelegations, AllowRule())
	t.Delegations.Roles = allDelegations
	return nil
}

// ReorderRules changes the order of delegations, and the new order is specified
// in `ruleNames []string`.
func (t *TargetsMetadata) ReorderRules(ruleNames []string) error {
	// Create a map of all existing delegations for quick look up
	rolesMap := make(map[string]*Delegation)

	// Create a set of current rules in metadata, skipping the allow rule
	currentRules := set.NewSet[string]()
	for _, delegation := range t.Delegations.Roles {
		if delegation.Name == tuf.AllowRuleName {
			continue
		}
		rolesMap[delegation.Name] = delegation
		currentRules.Add(delegation.Name)
	}

	specifiedRules := set.NewSet[string]()
	for _, name := range ruleNames {
		if specifiedRules.Has(name) {
			return fmt.Errorf("%w: '%s'", tuf.ErrDuplicatedRuleName, name)
		}
		specifiedRules.Add(name)
	}

	if !currentRules.Equal(specifiedRules) {
		onlyInSpecifiedRules := specifiedRules.Minus(currentRules)
		if onlyInSpecifiedRules.Len() != 0 {
			if onlyInSpecifiedRules.Has(tuf.AllowRuleName) {
				return fmt.Errorf("%w: do not specify allow rule", tuf.ErrCannotManipulateRulesWithGittufPrefix)
			}

			contents := onlyInSpecifiedRules.Contents()
			return fmt.Errorf("%w: rules '%s' do not exist in current rule file", tuf.ErrRuleNotFound, strings.Join(contents, ", "))
		}

		onlyInCurrentRules := currentRules.Minus(specifiedRules)
		if onlyInCurrentRules.Len() != 0 {
			contents := onlyInCurrentRules.Contents()
			return fmt.Errorf("%w: rules '%s' not specified", tuf.ErrMissingRules, strings.Join(contents, ", "))
		}
	}

	// Create newDelegations and set it in the targetsMetadata after adding allow rule
	newDelegations := make([]*Delegation, 0, len(rolesMap)+1)
	for _, ruleName := range ruleNames {
		newDelegations = append(newDelegations, rolesMap[ruleName])
	}
	newDelegations = append(newDelegations, AllowRule())
	t.Delegations.Roles = newDelegations
	return nil
}

// RemoveRule deletes a delegation entry from TargetsMetadata.
func (t *TargetsMetadata) RemoveRule(ruleName string) error {
	if strings.HasPrefix(ruleName, tuf.GittufPrefix) {
		return tuf.ErrCannotManipulateRulesWithGittufPrefix
	}

	allDelegations := t.Delegations.Roles
	updatedDelegations := []*Delegation{}

	for _, delegation := range allDelegations {
		if delegation.Name != ruleName {
			updatedDelegations = append(updatedDelegations, delegation)
		}
	}
	t.Delegations.Roles = updatedDelegations
	return nil
}

// GetPrincipals returns all the principals in the rule file.
func (t *TargetsMetadata) GetPrincipals() map[string]tuf.Principal {
	principals := map[string]tuf.Principal{}
	for id, principal := range t.Delegations.Principals {
		principals[id] = principal
	}

	return principals
}

// GetRules returns all the rules in the metadata.
func (t *TargetsMetadata) GetRules() []tuf.Rule {
	if t.Delegations == nil {
		return nil
	}

	rules := make([]tuf.Rule, 0, len(t.Delegations.Roles))
	for _, delegation := range t.Delegations.Roles {
		rules = append(rules, delegation)
	}

	return rules
}

// AddPrincipal adds a principal to the metadata.
//
// TODO: this isn't associated with a specific rule; with the removal of
// verify-commit and verify-tag, it may not make sense anymore
func (t *TargetsMetadata) AddPrincipal(principal tuf.Principal) error {
	return t.Delegations.addPrincipal(principal)
}

// RemovePrincipal removes a principal from the metadata.
func (t *TargetsMetadata) RemovePrincipal(principalID string) error {
	return t.Delegations.removePrincipal(principalID)
}

// Delegations defines the schema for specifying delegations in TUF's Targets
// metadata.
type Delegations struct {
	Principals map[string]tuf.Principal `json:"principals"`
	Roles      []*Delegation            `json:"roles"`
}

func (d *Delegations) UnmarshalJSON(data []byte) error {
	// this type _has_ to be a copy of Delegations, minus the use of
	// json.RawMessage in place of tuf.Principal
	type tempType struct {
		Principals map[string]json.RawMessage `json:"principals"`
		Roles      []*Delegation              `json:"roles"`
	}

	temp := &tempType{}
	if err := json.Unmarshal(data, temp); err != nil {
		return fmt.Errorf("unable to unmarshal json: %w", err)
	}

	d.Principals = make(map[string]tuf.Principal)
	for principalID, principalBytes := range temp.Principals {
		tempPrincipal := map[string]any{}
		if err := json.Unmarshal(principalBytes, &tempPrincipal); err != nil {
			return fmt.Errorf("unable to unmarshal json: %w", err)
		}

		if _, has := tempPrincipal["keyid"]; has {
			// this is *Key
			key := &Key{}
			if err := json.Unmarshal(principalBytes, key); err != nil {
				return fmt.Errorf("unable to unmarshal json: %w", err)
			}

			d.Principals[principalID] = key
			continue
		}

		if _, has := tempPrincipal["personID"]; has {
			// this is *Person
			person := &Person{}
			if err := json.Unmarshal(principalBytes, person); err != nil {
				return fmt.Errorf("unable to unmarshal json: %w", err)
			}

			d.Principals[principalID] = person
			continue
		}

		return fmt.Errorf("unrecognized principal type '%s'", string(principalBytes))
	}

	d.Roles = temp.Roles

	return nil
}

// addPrincipal adds a delegations key or person.  v02 supports Key and Person
// as principal types.
func (d *Delegations) addPrincipal(principal tuf.Principal) error {
	if d.Principals == nil {
		d.Principals = map[string]tuf.Principal{}
	}

	switch principal := principal.(type) {
	case *Key, *Person:
		d.Principals[principal.ID()] = principal
	default:
		return tuf.ErrInvalidPrincipalType
	}

	return nil
}

// removePrincipal removes a delegations key or person. v02 supports Key and
// Person as principal types.
func (d *Delegations) removePrincipal(principalID string) error {
	if d.Principals == nil {
		return tuf.ErrPrincipalNotFound
	}
	if principalID == "" {
		return tuf.ErrInvalidPrincipalID
	}
	for _, curRole := range d.Roles {
		if curRole.GetPrincipalIDs() != nil && curRole.GetPrincipalIDs().Has(principalID) {
			return tuf.ErrPrincipalStillInUse
		}
	}
	delete(d.Principals, principalID)
	return nil
}

// AllowRule returns the default, last rule for all policy files.
func AllowRule() *Delegation {
	return &Delegation{
		Name:        tuf.AllowRuleName,
		Paths:       []string{"*"},
		Terminating: true,
		Role: Role{
			Threshold: 1,
		},
	}
}

// Delegation defines the schema for a single delegation entry. It differs from
// the standard TUF schema by allowing a `custom` field to record details
// pertaining to the delegation. It implements the tuf.Rule interface.
type Delegation struct {
	Name        string           `json:"name"`
	Paths       []string         `json:"paths"`
	Terminating bool             `json:"terminating"`
	Custom      *json.RawMessage `json:"custom,omitempty"`
	Role
}

// ID returns the identifier of the delegation, its name.
func (d *Delegation) ID() string {
	return d.Name
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

// GetPrincipalIDs returns the identifiers of the principals that are listed as
// trusted by the rule.
func (d *Delegation) GetPrincipalIDs() *set.Set[string] {
	return d.PrincipalIDs
}

// GetThreshold returns the threshold of principals that must approve to meet
// the rule.
func (d *Delegation) GetThreshold() int {
	return d.Threshold
}

// IsLastTrustedInRuleFile indicates that subsequent rules in the rule file are
// not to be trusted if the current rule matches the namespace under
// verification (similar to TUF's terminating behavior). However, the current
// rule's delegated rules as well as other rules already in the queue are
// trusted.
func (d *Delegation) IsLastTrustedInRuleFile() bool {
	return d.Terminating
}

// GetProtectedNamespaces returns the set of namespaces protected by the
// delegation.
func (d *Delegation) GetProtectedNamespaces() []string {
	return d.Paths
}
