// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danwakefield/fnmatch"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/tuf"
)

const (
	targetsVersion = "http://gittuf.dev/policy/rule-file/v0.1"
)

// Hooks defines the schema that groups hooks by stage.
type Hooks struct {
	PreCommit map[string]Hook `json:"pre-commit"`
	PrePush   map[string]Hook `json:"pre-push"`
}

// TargetsMetadata defines the schema of TUF's Targets role.
type TargetsMetadata struct {
	Type        string         `json:"type"`
	Expires     string         `json:"expires"`
	Targets     map[string]any `json:"targets"`
	Hooks       Hooks          `json:"hooks"`
	Delegations *Delegations   `json:"delegations"`
}

// NewTargetsMetadata returns a new instance of TargetsMetadata.
func NewTargetsMetadata() *TargetsMetadata {
	return &TargetsMetadata{
		Type:        "targets",
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
	return targetsVersion
}

// Validate ensures the instance of TargetsMetadata matches gittuf expectations.
// func (t *TargetsMetadata) Validate() error {
// 	if len(t.Targets) != 0 {
// 		return ErrTargetsNotEmpty
// 	}
// 	return nil
// }

// AddRule adds a new delegation to TargetsMetadata.
func (t *TargetsMetadata) AddRule(ruleName string, authorizedPrincipalIDs, rulePatterns []string, threshold int) error {
	if ruleName == tuf.AllowRuleName {
		return tuf.ErrCannotManipulateAllowRule
	}

	for _, principalID := range authorizedPrincipalIDs {
		if _, has := t.Delegations.Keys[principalID]; !has {
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
			KeyIDs:    set.NewSetFromItems(authorizedPrincipalIDs...),
			Threshold: threshold,
		},
	}
	allDelegations = append(allDelegations[:len(allDelegations)-1], newDelegation, AllowRule())
	t.Delegations.Roles = allDelegations
	return nil
}

// UpdateRule is used to amend a delegation in TargetsMetadata.
func (t *TargetsMetadata) UpdateRule(ruleName string, authorizedPrincipalIDs, rulePatterns []string, threshold int) error {
	if ruleName == tuf.AllowRuleName {
		return tuf.ErrCannotManipulateAllowRule
	}

	for _, principalID := range authorizedPrincipalIDs {
		if _, has := t.Delegations.Keys[principalID]; !has {
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
				KeyIDs:    set.NewSetFromItems(authorizedPrincipalIDs...),
				Threshold: threshold,
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
				return fmt.Errorf("%w: do not specify allow rule", tuf.ErrCannotManipulateAllowRule)
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
	if ruleName == tuf.AllowRuleName {
		return tuf.ErrCannotManipulateAllowRule
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
	for id, key := range t.Delegations.Keys {
		principals[id] = key
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
	return t.Delegations.addKey(principal)
}

// AddHook adds the specified hook to the metadata.
func (t *TargetsMetadata) AddHook(stage, hookName, env string, hashes map[string]gitinterface.Hash, modules, principalIDs []string) error {
	for _, principalID := range principalIDs {
		if _, has := t.Delegations.Keys[principalID]; !has {
			return tuf.ErrPrincipalNotFound
		}
	}

	newHook := Hook{
		Name:         hookName,
		PrincipalIDs: set.NewSetFromItems(principalIDs...),
		Hashes:       hashes,
		Environment:  env,
		Modules:      modules,
	}

	switch stage {
	case "pre-commit":
		t.Hooks.PreCommit[hookName] = newHook
	case "pre-push":
		t.Hooks.PrePush[hookName] = newHook
	default:
		return tuf.ErrInvalidHookStage
	}

	return nil
}

// RemoveHook removes the hook specified by stage and hookName.
func (t *TargetsMetadata) RemoveHook(stage, hookName string) error {
	switch stage {
	case "pre-commit":
		delete(t.Hooks.PreCommit, hookName)
	case "pre-push":
		delete(t.Hooks.PrePush, hookName)
	default:
		return tuf.ErrInvalidHookStage
	}

	return nil
}

// GetHooks returns the hooks for the specified stage.
func (t *TargetsMetadata) GetHooks(stage string) (map[string]tuf.Applet, error) {
	var originMap map[string]Hook
	switch stage {
	case "pre-commit":
		originMap = t.Hooks.PreCommit
	case "pre-push":
		originMap = t.Hooks.PrePush
	default:
		return nil, tuf.ErrInvalidHookStage
	}

	var appletMap map[string]tuf.Applet

	for name, hook := range originMap {
		appletMap[name] = &hook
	}

	return appletMap, nil
}

// Delegations defines the schema for specifying delegations in TUF's Targets
// metadata.
type Delegations struct {
	Keys  map[string]*Key `json:"keys"`
	Roles []*Delegation   `json:"roles"`
}

// addKey adds a delegations key.
func (d *Delegations) addKey(key tuf.Principal) error {
	if d.Keys == nil {
		d.Keys = map[string]*Key{}
	}

	keyT, isKnownType := key.(*Key)
	if !isKnownType {
		return tuf.ErrInvalidPrincipalType
	}

	d.Keys[key.ID()] = keyT
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
	return d.Role.KeyIDs
}

// GetThreshold returns the threshold of principals that must approve to meet
// the rule.
func (d *Delegation) GetThreshold() int {
	return d.Role.Threshold
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

// Hook defines the schema for a hook.
type Hook struct {
	Name         string                       `json:"name"`
	PrincipalIDs *set.Set[string]             `json:"principals"`
	Hashes       map[string]gitinterface.Hash `json:"hashes"`
	Environment  string                       `json:"environment"`
	Modules      []string                     `json:"modules"`
}

// ID returns the identifier of the hook, its name.
func (h *Hook) ID() string {
	return h.Name
}

// GetPrincipalIDs returns the principals that must run this hook.
func (h *Hook) GetPrincipalIDs() *set.Set[string] {
	return h.PrincipalIDs
}

// GetHashes returns the hashes of the hook file.
func (h *Hook) GetHashes() map[string]gitinterface.Hash {
	return h.Hashes
}

// GetEnvironment returns the environment that the hook is to run in.
func (h *Hook) GetEnvironment() string {
	return h.Environment
}

// GetModules returns the Lua modules that the hook will have access to.
func (h *Hook) GetModules() []string {
	return h.Modules
}
