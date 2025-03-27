// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
)

const (
	RootVersion = "https://gittuf.dev/policy/root/v0.2"
)

// RootMetadata defines the schema of TUF's Root role.
type RootMetadata struct {
	Type                   string                     `json:"type"`
	Version                string                     `json:"schemaVersion"`
	Expires                string                     `json:"expires"`
	RepositoryLocation     string                     `json:"repositoryLocation,omitempty"`
	Principals             map[string]tuf.Principal   `json:"principals"`
	Roles                  map[string]Role            `json:"roles"`
	GitHubApprovalsTrusted bool                       `json:"githubApprovalsTrusted"`
	GlobalRules            []tuf.GlobalRule           `json:"globalRules,omitempty"`
	Propagations           []tuf.PropagationDirective `json:"propagations,omitempty"`
	MultiRepository        *MultiRepository           `json:"multiRepository,omitempty"`
	Hooks                  map[tuf.HookStage][]*Hook  `json:"hooks,omitempty"`
}

// NewRootMetadata returns a new instance of RootMetadata.
func NewRootMetadata() *RootMetadata {
	return &RootMetadata{
		Type:    "root",
		Version: RootVersion,
	}
}

// SetExpires sets the expiry date of the RootMetadata to the value passed in.
func (r *RootMetadata) SetExpires(expires string) {
	r.Expires = expires
}

// SchemaVersion returns the metadata schema version.
func (r *RootMetadata) SchemaVersion() string {
	return r.Version
}

// GetRepositoryLocation returns the canonical location of the Git repository.
func (r *RootMetadata) GetRepositoryLocation() string {
	return r.RepositoryLocation
}

// SetRepositoryLocation sets the specified repository location in the root
// metadata.
func (r *RootMetadata) SetRepositoryLocation(location string) {
	r.RepositoryLocation = location
}

// AddRootPrincipal adds the specified principal to the root metadata and
// authorizes the principal for the root role.
func (r *RootMetadata) AddRootPrincipal(principal tuf.Principal) error {
	if principal == nil {
		return tuf.ErrInvalidPrincipalType
	}

	// Add principal to metadata
	if err := r.addPrincipal(principal); err != nil {
		return err
	}

	rootRole, ok := r.Roles[tuf.RootRoleName]
	if !ok {
		// Create a new root role entry with this principal
		r.addRole(tuf.RootRoleName, Role{
			PrincipalIDs: set.NewSetFromItems(principal.ID()),
			Threshold:    1,
		})

		return nil
	}

	// Add principal ID to the root role if it's not already in it
	rootRole.PrincipalIDs.Add(principal.ID())
	r.Roles[tuf.RootRoleName] = rootRole
	return nil
}

// DeleteRootPrincipal removes principalID from the list of trusted Root
// principals in rootMetadata. It does not remove the principal entry itself as
// it does not check if other roles can be verified using the same principal.
func (r *RootMetadata) DeleteRootPrincipal(principalID string) error {
	rootRole, has := r.Roles[tuf.RootRoleName]
	if !has {
		return tuf.ErrInvalidRootMetadata
	}

	if rootRole.PrincipalIDs.Len() <= rootRole.Threshold {
		return tuf.ErrCannotMeetThreshold
	}

	rootRole.PrincipalIDs.Remove(principalID)
	r.Roles[tuf.RootRoleName] = rootRole
	return nil
}

// AddPrimaryRuleFilePrincipal adds the 'principal' as a trusted signer in
// 'rootMetadata' for the top level Targets role.
func (r *RootMetadata) AddPrimaryRuleFilePrincipal(principal tuf.Principal) error {
	if principal == nil {
		return tuf.ErrInvalidPrincipalType
	}

	// Add principal to the metadata file
	if err := r.addPrincipal(principal); err != nil {
		return err
	}

	targetsRole, ok := r.Roles[tuf.TargetsRoleName]
	if !ok {
		// Create a new targets role entry with this principal
		r.addRole(tuf.TargetsRoleName, Role{
			PrincipalIDs: set.NewSetFromItems(principal.ID()),
			Threshold:    1,
		})

		return nil
	}

	targetsRole.PrincipalIDs.Add(principal.ID())
	r.Roles[tuf.TargetsRoleName] = targetsRole

	return nil
}

// DeletePrimaryRuleFilePrincipal removes the principal matching 'principalID'
// from trusted principals for top level Targets role in 'rootMetadata'. Note:
// It doesn't remove the principal entry itself as it doesn't check if other
// roles can use the same principal.
func (r *RootMetadata) DeletePrimaryRuleFilePrincipal(principalID string) error {
	if principalID == "" {
		return tuf.ErrInvalidPrincipalID
	}

	targetsRole, ok := r.Roles[tuf.TargetsRoleName]
	if !ok {
		return tuf.ErrPrimaryRuleFileInformationNotFoundInRoot
	}

	if targetsRole.PrincipalIDs.Len() <= targetsRole.Threshold {
		return tuf.ErrCannotMeetThreshold
	}

	targetsRole.PrincipalIDs.Remove(principalID)
	r.Roles[tuf.TargetsRoleName] = targetsRole
	return nil
}

// AddGitHubAppPrincipal adds the 'principal' as a trusted principal in
// 'rootMetadata' for the special GitHub app role. This key is used to verify
// GitHub pull request approval attestation signatures.
func (r *RootMetadata) AddGitHubAppPrincipal(name string, principal tuf.Principal) error {
	if principal == nil {
		return tuf.ErrInvalidPrincipalType
	}

	// TODO: support multiple principals / threshold for app
	if err := r.addPrincipal(principal); err != nil {
		return err
	}
	role := Role{
		PrincipalIDs: set.NewSetFromItems(principal.ID()),
		Threshold:    1,
	}
	r.addRole(name, role) // AddRole replaces the specified role if it already exists
	return nil
}

// DeleteGitHubAppPrincipal removes the special GitHub app role from the root
// metadata.
func (r *RootMetadata) DeleteGitHubAppPrincipal(name string) {
	// TODO: support multiple principals / threshold for app
	delete(r.Roles, name)
}

// EnableGitHubAppApprovals sets GitHubApprovalsTrusted to true in the
// root metadata.
func (r *RootMetadata) EnableGitHubAppApprovals() {
	r.GitHubApprovalsTrusted = true
}

// DisableGitHubAppApprovals sets GitHubApprovalsTrusted to false in the root
// metadata.
func (r *RootMetadata) DisableGitHubAppApprovals() {
	r.GitHubApprovalsTrusted = false
}

// UpdateRootThreshold sets the threshold for the Root role.
func (r *RootMetadata) UpdateRootThreshold(threshold int) error {
	rootRole, ok := r.Roles[tuf.RootRoleName]
	if !ok {
		return tuf.ErrInvalidRootMetadata
	}

	if rootRole.PrincipalIDs.Len() < threshold {
		return tuf.ErrCannotMeetThreshold
	}
	rootRole.Threshold = threshold
	r.Roles[tuf.RootRoleName] = rootRole
	return nil
}

// UpdatePrimaryRuleFileThreshold sets the threshold for the top level Targets
// role.
func (r *RootMetadata) UpdatePrimaryRuleFileThreshold(threshold int) error {
	targetsRole, ok := r.Roles[tuf.TargetsRoleName]
	if !ok {
		return tuf.ErrPrimaryRuleFileInformationNotFoundInRoot
	}

	if targetsRole.PrincipalIDs.Len() < threshold {
		return tuf.ErrCannotMeetThreshold
	}
	targetsRole.Threshold = threshold
	r.Roles[tuf.TargetsRoleName] = targetsRole
	return nil
}

// GetPrincipals returns all the principals in the root metadata.
func (r *RootMetadata) GetPrincipals() map[string]tuf.Principal {
	return r.Principals
}

// GetRootThreshold returns the threshold of principals that must sign the root
// of trust metadata.
func (r *RootMetadata) GetRootThreshold() (int, error) {
	role, hasRole := r.Roles[tuf.RootRoleName]
	if !hasRole {
		return -1, tuf.ErrInvalidRootMetadata
	}

	return role.Threshold, nil
}

// GetRootPrincipals returns the principals trusted for the root of trust
// metadata.
func (r *RootMetadata) GetRootPrincipals() ([]tuf.Principal, error) {
	role, hasRole := r.Roles[tuf.RootRoleName]
	if !hasRole {
		return nil, tuf.ErrInvalidRootMetadata
	}

	principals := make([]tuf.Principal, 0, role.PrincipalIDs.Len())
	for _, id := range role.PrincipalIDs.Contents() {
		principals = append(principals, r.Principals[id])
	}

	return principals, nil
}

// GetPrimaryRuleFileThreshold returns the threshold of principals that must
// sign the primary rule file.
func (r *RootMetadata) GetPrimaryRuleFileThreshold() (int, error) {
	role, hasRole := r.Roles[tuf.TargetsRoleName]
	if !hasRole {
		return -1, tuf.ErrPrimaryRuleFileInformationNotFoundInRoot
	}

	return role.Threshold, nil
}

// GetPrimaryRuleFilePrincipals returns the principals trusted for the primary
// rule file.
func (r *RootMetadata) GetPrimaryRuleFilePrincipals() ([]tuf.Principal, error) {
	role, hasRole := r.Roles[tuf.TargetsRoleName]
	if !hasRole {
		return nil, tuf.ErrPrimaryRuleFileInformationNotFoundInRoot
	}

	principals := make([]tuf.Principal, 0, role.PrincipalIDs.Len())
	for _, id := range role.PrincipalIDs.Contents() {
		principals = append(principals, r.Principals[id])
	}

	return principals, nil
}

// IsGitHubAppApprovalTrusted indicates if the GitHub app is trusted.
//
// TODO: this needs to be generalized across tools
func (r *RootMetadata) IsGitHubAppApprovalTrusted() bool {
	return r.GitHubApprovalsTrusted
}

// GetGitHubAppPrincipals returns the principals trusted for the GitHub app
// attestations.
//
// TODO: this needs to be generalized across tools
func (r *RootMetadata) GetGitHubAppPrincipals() ([]tuf.Principal, error) {
	role, hasRole := r.Roles[tuf.GitHubAppRoleName]
	if !hasRole {
		return nil, tuf.ErrGitHubAppInformationNotFoundInRoot
	}

	principals := make([]tuf.Principal, 0, role.PrincipalIDs.Len())
	for _, id := range role.PrincipalIDs.Contents() {
		principals = append(principals, r.Principals[id])
	}

	return principals, nil
}

func (r *RootMetadata) UnmarshalJSON(data []byte) error {
	// this type _has_ to be a copy of RootMetadata, minus the use of
	// json.RawMessage in place of tuf interfaces
	type tempType struct {
		Type                   string                     `json:"type"`
		Version                string                     `json:"schemaVersion"`
		Expires                string                     `json:"expires"`
		RepositoryLocation     string                     `json:"repositoryLocation,omitempty"`
		Principals             map[string]json.RawMessage `json:"principals"`
		Roles                  map[string]Role            `json:"roles"`
		GitHubApprovalsTrusted bool                       `json:"githubApprovalsTrusted"`
		GlobalRules            []json.RawMessage          `json:"globalRules,omitempty"`
		Propagations           []json.RawMessage          `json:"propagations,omitempty"`
		MultiRepository        *MultiRepository           `json:"multiRepository,omitempty"`
		Hooks                  map[tuf.HookStage][]*Hook  `json:"hooks,omitempty"`
	}

	temp := &tempType{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("unable to unmarshal json: %w", err)
	}

	r.Type = temp.Type
	r.Version = temp.Version
	r.Expires = temp.Expires
	r.RepositoryLocation = temp.RepositoryLocation

	r.Principals = make(map[string]tuf.Principal)
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

			r.Principals[principalID] = key
			continue
		}

		if _, has := tempPrincipal["personID"]; has {
			// this is *Person
			person := &Person{}
			if err := json.Unmarshal(principalBytes, person); err != nil {
				return fmt.Errorf("unable to unmarshal json: %w", err)
			}

			r.Principals[principalID] = person
			continue
		}

		return fmt.Errorf("unrecognized principal type '%s'", string(principalBytes))
	}

	r.Roles = temp.Roles
	r.GitHubApprovalsTrusted = temp.GitHubApprovalsTrusted

	r.GlobalRules = []tuf.GlobalRule{}
	for _, globalRuleBytes := range temp.GlobalRules {
		tempGlobalRule := map[string]any{}
		if err := json.Unmarshal(globalRuleBytes, &tempGlobalRule); err != nil {
			return fmt.Errorf("unable to unmarshal json: %w", err)
		}

		switch tempGlobalRule["type"] {
		case tuf.GlobalRuleThresholdType:
			globalRule := &GlobalRuleThreshold{}
			if err := json.Unmarshal(globalRuleBytes, globalRule); err != nil {
				return fmt.Errorf("unable to unmarshal json: %w", err)
			}

			r.GlobalRules = append(r.GlobalRules, globalRule)

		case tuf.GlobalRuleBlockForcePushesType:
			globalRule := &GlobalRuleBlockForcePushes{}
			if err := json.Unmarshal(globalRuleBytes, globalRule); err != nil {
				return fmt.Errorf("unable to unmarshal json: %w", err)
			}

			r.GlobalRules = append(r.GlobalRules, globalRule)

		default:
			return tuf.ErrUnknownGlobalRuleType
		}
	}

	r.Propagations = []tuf.PropagationDirective{}
	for _, propagationDirectiveBytes := range temp.Propagations {
		propagationDirective := &PropagationDirective{}
		if err := json.Unmarshal(propagationDirectiveBytes, propagationDirective); err != nil {
			return fmt.Errorf("unable to unmarshal json for propagation directive: %w", err)
		}

		r.Propagations = append(r.Propagations, propagationDirective)
	}

	r.MultiRepository = temp.MultiRepository

	r.Hooks = temp.Hooks

	return nil
}

// AddGlobalRule adds a new global rule to RootMetadata.
func (r *RootMetadata) AddGlobalRule(globalRule tuf.GlobalRule) error {
	allGlobalRules := r.GlobalRules
	if allGlobalRules == nil {
		allGlobalRules = []tuf.GlobalRule{}
	}

	// check for duplicates
	for _, rule := range allGlobalRules {
		if rule.GetName() == globalRule.GetName() {
			return tuf.ErrGlobalRuleAlreadyExists
		}
	}

	allGlobalRules = append(allGlobalRules, globalRule)
	r.GlobalRules = allGlobalRules
	return nil
}

// DeleteGlobalRule removes the specified global rule from the RootMetadata.
func (r *RootMetadata) DeleteGlobalRule(ruleName string) error {
	allGlobalRules := r.GlobalRules
	updatedGlobalRules := []tuf.GlobalRule{}

	if len(allGlobalRules) == 0 {
		return tuf.ErrGlobalRuleNotFound
	}

	for _, rule := range allGlobalRules {
		if rule.GetName() != ruleName {
			updatedGlobalRules = append(updatedGlobalRules, rule)
		}
	}
	r.GlobalRules = updatedGlobalRules
	return nil
}

// UpdateGlobalRule updates the specified global rule from the RootMetadata.
func (r *RootMetadata) UpdateGlobalRule(globalRule tuf.GlobalRule) error {
	allGlobalRules := r.GlobalRules
	updatedGlobalRules := []tuf.GlobalRule{}
	found := false

	if len(allGlobalRules) == 0 {
		return tuf.ErrGlobalRuleNotFound
	}

	for _, oldGlobalRule := range allGlobalRules {
		if oldGlobalRule.GetName() == globalRule.GetName() {
			switch oldGlobalRule.(type) {
			case *GlobalRuleThreshold:
				if _, ok := globalRule.(*GlobalRuleThreshold); !ok {
					return tuf.ErrCannotUpdateGlobalRuleType
				}
			case *GlobalRuleBlockForcePushes:
				if _, ok := globalRule.(*GlobalRuleBlockForcePushes); !ok {
					return tuf.ErrCannotUpdateGlobalRuleType
				}
			}
			found = true
			updatedGlobalRules = append(updatedGlobalRules, globalRule)
		} else {
			updatedGlobalRules = append(updatedGlobalRules, oldGlobalRule)
		}
	}

	if !found {
		return tuf.ErrGlobalRuleNotFound
	}

	r.GlobalRules = updatedGlobalRules

	return nil
}

// GetGlobalRules returns all the global rules in the root metadata.
func (r *RootMetadata) GetGlobalRules() []tuf.GlobalRule {
	return r.GlobalRules
}

// AddPropagationDirective adds a propagation directive to the root metadata.
func (r *RootMetadata) AddPropagationDirective(directive tuf.PropagationDirective) error {
	// TODO: handle duplicates / updates
	r.Propagations = append(r.Propagations, directive)
	return nil
}

// GetPropagationDirectives returns the propagation directives found in the root
// metadata.
func (r *RootMetadata) GetPropagationDirectives() []tuf.PropagationDirective {
	return r.Propagations
}

// DeletePropagationDirective removes a propagation directive from the root
// metadata.
func (r *RootMetadata) DeletePropagationDirective(name string) error {
	index := -1
	for i, directive := range r.Propagations {
		if directive.GetName() == name {
			index = i
			break
		}
	}

	if index == -1 {
		return tuf.ErrPropagationDirectiveNotFound
	}

	r.Propagations = append(r.Propagations[:index], r.Propagations[index+1:]...)
	return nil
}

// IsController indicates if the repository serves as the controller for a
// multi-repository gittuf network.
func (r *RootMetadata) IsController() bool {
	if r.MultiRepository == nil {
		return false
	}

	return r.MultiRepository.IsController()
}

// EnableController marks the current repository as a controller repository.
func (r *RootMetadata) EnableController() error {
	if r.MultiRepository == nil {
		r.MultiRepository = &MultiRepository{}
	}

	r.MultiRepository.Controller = true
	return nil // TODO: what if it's already a controller? noop?
}

// DisableController marks the current repository as not-a-controller.
func (r *RootMetadata) DisableController() error {
	if r.MultiRepository == nil {
		// nothing to do
		return nil
	}

	r.MultiRepository.Controller = false
	// TODO: should we remove the network repository entries?
	return nil
}

// AddControllerRepository adds the specified repository as a controller for the
// current repository.
func (r *RootMetadata) AddControllerRepository(name, location string, initialRootPrincipals []tuf.Principal) error {
	if r.MultiRepository == nil {
		r.MultiRepository = &MultiRepository{ControllerRepositories: []*OtherRepository{}}
	}

	for _, repo := range r.MultiRepository.ControllerRepositories {
		if repo.Name == name || repo.Location == location {
			return tuf.ErrDuplicateControllerRepository
		}
	}

	newKeyIDs := make([]tuf.Principal, 0, len(initialRootPrincipals))
	for _, principal := range initialRootPrincipals {
		switch p := principal.(type) {
		case *Key:
			newKeyIDs = append(newKeyIDs, p)
		case *Person:
			// don't need to be checked for duplicates skip
		default:
			return tuf.ErrInvalidPrincipalType
		}
	}

	for _, repo := range r.MultiRepository.ControllerRepositories {
		existingKeyIDSet := set.NewSet[string]()
		for _, existingPrincipal := range repo.InitialRootPrincipals {
			if key, isKey := existingPrincipal.(*Key); isKey {
				existingKeyIDSet.Add(key.KeyID)
			}
		}

		newKeyIDSet := set.NewSet[string]()
		for _, principal := range newKeyIDs {
			if key, isKey := principal.(*Key); isKey {
				newKeyIDSet.Add(key.KeyID)
			}
		}

		if newKeyIDSet.Equal(existingKeyIDSet) {
			return tuf.ErrDuplicateControllerRepository
		}
	}

	otherRepository := &OtherRepository{
		Name:                  name,
		Location:              location,
		InitialRootPrincipals: make([]tuf.Principal, 0, len(initialRootPrincipals)),
	}

	for _, principal := range initialRootPrincipals {
		switch p := principal.(type) {
		case *Key:
			otherRepository.InitialRootPrincipals = append(otherRepository.InitialRootPrincipals, p)
		case *Person:
			otherRepository.InitialRootPrincipals = append(otherRepository.InitialRootPrincipals, p)
		default:
			return tuf.ErrInvalidPrincipalType
		}
	}

	r.MultiRepository.ControllerRepositories = append(r.MultiRepository.ControllerRepositories, otherRepository)

	// Add the controller as a repository whose policy contents must be
	// propagated into this repository
	policyPropagationName := fmt.Sprintf("%s-%s-policy", tuf.GittufControllerPrefix, name)
	policyPropagationLocation := path.Join(tuf.GittufControllerPrefix, name)

	policyStagingPropagationName := fmt.Sprintf("%s-%s-policy-staging", tuf.GittufControllerPrefix, name)
	policyStagingPropagationLocation := path.Join(tuf.GittufControllerPrefix, name)

	if err := r.AddPropagationDirective(NewPropagationDirective(policyStagingPropagationName, location, "refs/gittuf/policy-staging", "refs/gittuf/policy-staging", policyStagingPropagationLocation)); err != nil {
		return err
	}
	return r.AddPropagationDirective(NewPropagationDirective(policyPropagationName, location, "refs/gittuf/policy", "refs/gittuf/policy", policyPropagationLocation))
}

// AddNetworkRepository adds the specified repository as part of the network for
// which the current repository is a controller. The current repository must be
// marked as a controller before this can be used.
func (r *RootMetadata) AddNetworkRepository(name, location string, initialRootPrincipals []tuf.Principal) error {
	if r.MultiRepository == nil || !r.MultiRepository.Controller {
		// EnableController must be called first
		return tuf.ErrNotAControllerRepository
	}

	if r.MultiRepository.NetworkRepositories == nil {
		r.MultiRepository.NetworkRepositories = []*OtherRepository{}
	}

	for _, repo := range r.MultiRepository.NetworkRepositories {
		if repo.Name == name || repo.Location == location {
			return tuf.ErrDuplicateNetworkRepository
		}
	}

	newKeyIDs := make([]tuf.Principal, 0, len(initialRootPrincipals))
	for _, principal := range initialRootPrincipals {
		switch p := principal.(type) {
		case *Key:
			newKeyIDs = append(newKeyIDs, p)
		case *Person:
			// don't need to be checked for duplicates skip
		default:
			return tuf.ErrInvalidPrincipalType
		}
	}

	for _, repo := range r.MultiRepository.NetworkRepositories {
		existingKeyIDSet := set.NewSet[string]()
		for _, existingPrincipal := range repo.InitialRootPrincipals {
			if key, isKey := existingPrincipal.(*Key); isKey {
				existingKeyIDSet.Add(key.KeyID)
			}
		}

		newKeyIDSet := set.NewSet[string]()
		for _, principal := range newKeyIDs {
			if key, isKey := principal.(*Key); isKey {
				newKeyIDSet.Add(key.KeyID)
			}
		}

		if newKeyIDSet.Equal(existingKeyIDSet) {
			return tuf.ErrDuplicateNetworkRepository
		}
	}

	otherRepository := &OtherRepository{
		Name:                  name,
		Location:              location,
		InitialRootPrincipals: make([]tuf.Principal, 0, len(initialRootPrincipals)),
	}

	for _, principal := range initialRootPrincipals {
		switch p := principal.(type) {
		case *Key:
			otherRepository.InitialRootPrincipals = append(otherRepository.InitialRootPrincipals, p)
		case *Person:
			otherRepository.InitialRootPrincipals = append(otherRepository.InitialRootPrincipals, p)
		default:
			return tuf.ErrInvalidPrincipalType
		}
	}

	r.MultiRepository.NetworkRepositories = append(r.MultiRepository.NetworkRepositories, otherRepository)

	return nil
}

// GetControllerRepositories returns the repositories that serve as the
// controllers for the networks the current repository is a part of.
func (r *RootMetadata) GetControllerRepositories() []tuf.OtherRepository {
	if r.MultiRepository == nil {
		return nil
	}

	return r.MultiRepository.GetControllerRepositories()
}

// GetNetworkRepositories returns the repositories that are part of the network
// for which the current repository is a controller. IsController must return
// true for this to be set.
func (r *RootMetadata) GetNetworkRepositories() []tuf.OtherRepository {
	if r.MultiRepository == nil {
		return nil
	}

	return r.MultiRepository.GetNetworkRepositories()
}

// addPrincipal adds a principal to the RootMetadata instance.  v02 of the
// metadata supports Key and Person as supported principal types.
func (r *RootMetadata) addPrincipal(principal tuf.Principal) error {
	if r.Principals == nil {
		r.Principals = map[string]tuf.Principal{}
	}
	switch principal := principal.(type) {
	case *Key, *Person:
		r.Principals[principal.ID()] = principal
	default:
		return tuf.ErrInvalidPrincipalType
	}

	return nil
}

// addRole adds a role object and associates it with roleName in the
// RootMetadata instance.
func (r *RootMetadata) addRole(roleName string, role Role) {
	if r.Roles == nil {
		r.Roles = map[string]Role{}
	}

	r.Roles[roleName] = role
}

type GlobalRuleThreshold = tufv01.GlobalRuleThreshold
type GlobalRuleBlockForcePushes = tufv01.GlobalRuleBlockForcePushes

var NewGlobalRuleThreshold = tufv01.NewGlobalRuleThreshold
var NewGlobalRuleBlockForcePushes = tufv01.NewGlobalRuleBlockForcePushes

type PropagationDirective = tufv01.PropagationDirective

func NewPropagationDirective(name, upstreamRepository, upstreamReference, downstreamReference, downstreamPath string) tuf.PropagationDirective {
	return &PropagationDirective{
		Name:                name,
		UpstreamRepository:  upstreamRepository,
		UpstreamReference:   upstreamReference,
		DownstreamReference: downstreamReference,
		DownstreamPath:      downstreamPath,
	}
}

type MultiRepository struct {
	Controller             bool               `json:"controller"`
	ControllerRepositories []*OtherRepository `json:"controllerRepositories,omitempty"`
	NetworkRepositories    []*OtherRepository `json:"networkRepositories,omitempty"`
}

func (m *MultiRepository) IsController() bool {
	return m.Controller
}

func (m *MultiRepository) GetControllerRepositories() []tuf.OtherRepository {
	controllerRepositories := []tuf.OtherRepository{}
	for _, repository := range m.ControllerRepositories {
		controllerRepositories = append(controllerRepositories, repository)
	}
	return controllerRepositories
}

func (m *MultiRepository) GetNetworkRepositories() []tuf.OtherRepository {
	if !m.Controller {
		return nil
	}

	networkRepositories := []tuf.OtherRepository{}
	for _, repository := range m.NetworkRepositories {
		networkRepositories = append(networkRepositories, repository)
	}
	return networkRepositories
}

type OtherRepository struct {
	Name                  string          `json:"name"`
	Location              string          `json:"location"`
	InitialRootPrincipals []tuf.Principal `json:"initialRootPrincipals"`
}

func (o *OtherRepository) GetName() string {
	return o.Name
}

func (o *OtherRepository) GetLocation() string {
	return o.Location
}

func (o *OtherRepository) GetInitialRootPrincipals() []tuf.Principal {
	return o.InitialRootPrincipals
}

func (o *OtherRepository) UnmarshalJSON(data []byte) error {
	type tempType struct {
		Name                  string            `json:"name"`
		Location              string            `json:"location"`
		InitialRootPrincipals []json.RawMessage `json:"initialRootPrincipals"`
	}

	temp := &tempType{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("unable to unmarshal json: %w", err)
	}

	o.Name = temp.Name
	o.Location = temp.Location

	o.InitialRootPrincipals = make([]tuf.Principal, 0, len(temp.InitialRootPrincipals))
	for _, principalBytes := range temp.InitialRootPrincipals {
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

			o.InitialRootPrincipals = append(o.InitialRootPrincipals, key)
			continue
		}

		if _, has := tempPrincipal["personID"]; has {
			// this is *Person
			person := &Person{}
			if err := json.Unmarshal(principalBytes, person); err != nil {
				return fmt.Errorf("unable to unmarshal json: %w", err)
			}

			o.InitialRootPrincipals = append(o.InitialRootPrincipals, person)
			continue
		}

		return fmt.Errorf("unrecognized principal type '%s'", string(principalBytes))
	}

	return nil
}

type Hook = tufv01.Hook

// AddHook adds the specified hook to the metadata.
func (r *RootMetadata) AddHook(stages []tuf.HookStage, hookName string, principalIDs []string, hashes map[string]string, environment tuf.HookEnvironment, modules []string) (tuf.Hook, error) {
	// TODO: Check if principal exists in RootMetadata/TargetsMetadata

	newHook := &Hook{
		Name:         hookName,
		PrincipalIDs: set.NewSetFromItems(principalIDs...),
		Hashes:       hashes,
		Environment:  environment,
		Modules:      modules,
	}

	if r.Hooks == nil {
		r.Hooks = map[tuf.HookStage][]*Hook{}
	}

	for _, stage := range stages {
		if err := stage.IsValid(); err != nil {
			return nil, err
		}
		if r.Hooks[stage] == nil {
			r.Hooks[stage] = []*Hook{}
		} else {
			for _, existingHook := range r.Hooks[stage] {
				if existingHook.Name == hookName {
					return nil, tuf.ErrDuplicatedHookName
				}
			}
		}

		r.Hooks[stage] = append(r.Hooks[stage], newHook)
	}

	return tuf.Hook(newHook), nil
}

// RemoveHook removes the hook specified by stage and hookName.
func (r *RootMetadata) RemoveHook(stages []tuf.HookStage, hookName string) error {
	if r.Hooks == nil {
		return tuf.ErrNoHooksDefined
	}

	for _, stage := range stages {
		hooks := []*Hook{}
		for _, hook := range r.Hooks[stage] {
			if hook.Name != hookName {
				hooks = append(hooks, hook)
			}
		}

		r.Hooks[stage] = hooks
	}

	return nil
}

// GetHooks returns the hooks for the specified stage.
func (r *RootMetadata) GetHooks(stage tuf.HookStage) ([]tuf.Hook, error) {
	if r.Hooks == nil {
		return nil, tuf.ErrNoHooksDefined
	}

	hooks := []tuf.Hook{}
	for _, hook := range r.Hooks[stage] {
		hooks = append(hooks, hook)
	}
	return hooks, nil
}
