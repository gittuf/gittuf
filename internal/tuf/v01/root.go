// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"encoding/json"
	"fmt"
	"path"
	"strings"

	"github.com/danwakefield/fnmatch"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/tuf"
)

const (
	rootVersion = "https://gittuf.dev/policy/root/v0.1"
)

// RootMetadata defines the schema of TUF's Root role.
type RootMetadata struct {
	Type                   string                     `json:"type"`
	Expires                string                     `json:"expires"`
	RepositoryLocation     string                     `json:"repositoryLocation,omitempty"`
	Keys                   map[string]*Key            `json:"keys"`
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
		Type: "root",
	}
}

// SetExpires sets the expiry date of the RootMetadata to the value passed in.
func (r *RootMetadata) SetExpires(expires string) {
	r.Expires = expires
}

// SchemaVersion returns the metadata schema version.
func (r *RootMetadata) SchemaVersion() string {
	return rootVersion
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

// AddRootPrincipal adds the specified key to the root metadata and authorizes the key
// for the root role.
func (r *RootMetadata) AddRootPrincipal(key tuf.Principal) error {
	if key == nil {
		return tuf.ErrInvalidPrincipalType
	}

	// Add key to metadata
	if err := r.addKey(key); err != nil {
		return err
	}

	if _, ok := r.Roles[tuf.RootRoleName]; !ok {
		// Create a new root role entry with this key
		r.addRole(tuf.RootRoleName, Role{
			KeyIDs:    set.NewSetFromItems(key.ID()),
			Threshold: 1,
		})

		return nil
	}

	// Add key ID to the root role if it's not already in it
	rootRole := r.Roles[tuf.RootRoleName]
	rootRole.KeyIDs.Add(key.ID())
	r.Roles[tuf.RootRoleName] = rootRole
	return nil
}

// DeleteRootPrincipal removes keyID from the list of trusted Root public keys
// in rootMetadata. It does not remove the key entry itself as it does not check
// if other roles can be verified using the same key.
func (r *RootMetadata) DeleteRootPrincipal(keyID string) error {
	if _, ok := r.Roles[tuf.RootRoleName]; !ok {
		return tuf.ErrInvalidRootMetadata
	}

	rootRole := r.Roles[tuf.RootRoleName]
	if rootRole.KeyIDs.Len() <= rootRole.Threshold {
		return tuf.ErrCannotMeetThreshold
	}

	rootRole.KeyIDs.Remove(keyID)
	r.Roles[tuf.RootRoleName] = rootRole
	return nil
}

// AddPrimaryRuleFilePrincipal adds the 'targetsKey' as a trusted public key in
// 'rootMetadata' for the top level Targets role.
func (r *RootMetadata) AddPrimaryRuleFilePrincipal(key tuf.Principal) error {
	if key == nil {
		return tuf.ErrInvalidPrincipalType
	}

	// Add key to the metadata file
	if err := r.addKey(key); err != nil {
		return err
	}

	if _, ok := r.Roles[tuf.TargetsRoleName]; !ok {
		// Create a new targets role entry with this key
		r.addRole(tuf.TargetsRoleName, Role{
			KeyIDs:    set.NewSetFromItems(key.ID()),
			Threshold: 1,
		})

		return nil
	}

	targetsRole := r.Roles[tuf.TargetsRoleName]
	targetsRole.KeyIDs.Add(key.ID())
	r.Roles[tuf.TargetsRoleName] = targetsRole

	return nil
}

// DeletePrimaryRuleFilePrincipal removes the key matching 'keyID' from trusted
// public keys for top level Targets role in 'rootMetadata'. Note: It doesn't
// remove the key entry itself as it doesn't check if other roles can use the
// same key.
func (r *RootMetadata) DeletePrimaryRuleFilePrincipal(keyID string) error {
	if keyID == "" {
		return tuf.ErrInvalidPrincipalID
	}

	targetsRole, ok := r.Roles[tuf.TargetsRoleName]
	if !ok {
		return tuf.ErrPrimaryRuleFileInformationNotFoundInRoot
	}

	if targetsRole.KeyIDs.Len() <= targetsRole.Threshold {
		return tuf.ErrCannotMeetThreshold
	}

	targetsRole.KeyIDs.Remove(keyID)
	r.Roles[tuf.TargetsRoleName] = targetsRole
	return nil
}

// AddGitHubAppPrincipal adds the 'appKey' as a trusted public key in
// 'rootMetadata' for the special GitHub app role. This key is used to verify
// GitHub pull request approval attestation signatures.
func (r *RootMetadata) AddGitHubAppPrincipal(name string, key tuf.Principal) error {
	if key == nil {
		return tuf.ErrInvalidPrincipalType
	}

	// TODO: support multiple keys / threshold for app
	if err := r.addKey(key); err != nil {
		return err
	}
	role := Role{
		KeyIDs:    set.NewSetFromItems(key.ID()),
		Threshold: 1,
	}
	r.addRole(name, role) // AddRole replaces the specified role if it already exists
	return nil
}

// DeleteGitHubAppPrincipal removes the special GitHub app role from the root
// metadata.
func (r *RootMetadata) DeleteGitHubAppPrincipal(name string) {
	// TODO: support multiple keys / threshold for app
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

	if rootRole.KeyIDs.Len() < threshold {
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

	if targetsRole.KeyIDs.Len() < threshold {
		return tuf.ErrCannotMeetThreshold
	}
	targetsRole.Threshold = threshold
	r.Roles[tuf.TargetsRoleName] = targetsRole
	return nil
}

// GetPrincipals returns all the principals in the root metadata.
func (r *RootMetadata) GetPrincipals() map[string]tuf.Principal {
	principals := map[string]tuf.Principal{}
	for id, key := range r.Keys {
		principals[id] = key
	}
	return principals
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

	principals := make([]tuf.Principal, 0, role.KeyIDs.Len())
	for _, id := range role.KeyIDs.Contents() {
		key, has := r.Keys[id]
		if !has {
			return nil, tuf.ErrInvalidPrincipalType
		}

		principals = append(principals, key)
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

	principals := make([]tuf.Principal, 0, role.KeyIDs.Len())
	for _, id := range role.KeyIDs.Contents() {
		key, has := r.Keys[id]
		if !has {
			return nil, tuf.ErrInvalidPrincipalType
		}

		principals = append(principals, key)
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

	principals := make([]tuf.Principal, 0, role.KeyIDs.Len())
	for _, id := range role.KeyIDs.Contents() {
		key, has := r.Keys[id]
		if !has {
			return nil, tuf.ErrInvalidPrincipalType
		}

		principals = append(principals, key)
	}

	return principals, nil
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

func (r *RootMetadata) GetGlobalRules() []tuf.GlobalRule {
	return r.GlobalRules
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
		key, isKey := principal.(*Key)
		if !isKey {
			return tuf.ErrInvalidPrincipalType
		}
		newKeyIDs = append(newKeyIDs, key)
	}

	newKeyIDsSet := set.NewSet[string]()
	for _, principal := range newKeyIDs {
		newKeyIDsSet.Add(principal.ID())
	}

	for _, repo := range r.MultiRepository.ControllerRepositories {
		existingKeyIDs := set.NewSet[string]()
		for _, existingPrincipal := range repo.InitialRootPrincipals {
			existingKeyIDs.Add(existingPrincipal.KeyID)
		}
		if existingKeyIDs.Equal(newKeyIDsSet) {
			return tuf.ErrDuplicateControllerRepository
		}
	}

	otherRepository := &OtherRepository{
		Name:                  name,
		Location:              location,
		InitialRootPrincipals: make([]*Key, 0, len(initialRootPrincipals)),
	}

	for _, principal := range initialRootPrincipals {
		key := principal.(*Key)
		otherRepository.InitialRootPrincipals = append(otherRepository.InitialRootPrincipals, key)
	}

	r.MultiRepository.ControllerRepositories = append(r.MultiRepository.ControllerRepositories, otherRepository)

	// Add the controller as a repository whose policy contents must be
	// propagated into this repository
	propagationName := fmt.Sprintf("%s-%s", tuf.GittufControllerPrefix, name)
	propagationLocation := path.Join(tuf.GittufControllerPrefix, name)
	return r.AddPropagationDirective(NewPropagationDirective(propagationName, location, "refs/gittuf/policy", "refs/gittuf/policy", propagationLocation))
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
		key, isKey := principal.(*Key)
		if !isKey {
			return tuf.ErrInvalidPrincipalType
		}
		newKeyIDs = append(newKeyIDs, key)
	}

	newKeyIDsSet := set.NewSet[string]()
	for _, principal := range newKeyIDs {
		newKeyIDsSet.Add(principal.ID())
	}

	for _, repo := range r.MultiRepository.NetworkRepositories {
		existingKeyIDs := set.NewSet[string]()
		for _, existingPrincipal := range repo.InitialRootPrincipals {
			existingKeyIDs.Add(existingPrincipal.KeyID)
		}

		if existingKeyIDs.Equal(newKeyIDsSet) {
			return tuf.ErrDuplicateNetworkRepository
		}
	}

	otherRepository := &OtherRepository{
		Name:                  name,
		Location:              location,
		InitialRootPrincipals: make([]*Key, 0, len(initialRootPrincipals)),
	}

	for _, principal := range initialRootPrincipals {
		key := principal.(*Key)
		otherRepository.InitialRootPrincipals = append(otherRepository.InitialRootPrincipals, key)
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

func (r *RootMetadata) UnmarshalJSON(data []byte) error {
	// this type _has_ to be a copy of RootMetadata, minus the use of
	// json.RawMessage for tuf interfaces
	type tempType struct {
		Type                   string                    `json:"type"`
		Expires                string                    `json:"expires"`
		RepositoryLocation     string                    `json:"repositoryLocation,omitempty"`
		Keys                   map[string]*Key           `json:"keys"`
		Roles                  map[string]Role           `json:"roles"`
		GitHubApprovalsTrusted bool                      `json:"githubApprovalsTrusted"`
		GlobalRules            []json.RawMessage         `json:"globalRules,omitempty"`
		Propagations           []json.RawMessage         `json:"propagations,omitempty"`
		MultiRepository        *MultiRepository          `json:"multiRepository,omitempty"`
		Hooks                  map[tuf.HookStage][]*Hook `json:"hooks,omitempty"`
	}

	temp := &tempType{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("unable to unmarshal json: %w", err)
	}

	r.Type = temp.Type
	r.Expires = temp.Expires
	r.RepositoryLocation = temp.RepositoryLocation
	r.Keys = temp.Keys
	r.Roles = temp.Roles
	r.GitHubApprovalsTrusted = temp.GitHubApprovalsTrusted

	r.GlobalRules = []tuf.GlobalRule{}
	for _, globalRuleBytes := range temp.GlobalRules {
		tempGlobalRule := map[string]any{}
		if err := json.Unmarshal(globalRuleBytes, &tempGlobalRule); err != nil {
			return fmt.Errorf("unable to unmarshal json for global rule: %w", err)
		}

		switch tempGlobalRule["type"] {
		case tuf.GlobalRuleThresholdType:
			globalRule := &GlobalRuleThreshold{}
			if err := json.Unmarshal(globalRuleBytes, globalRule); err != nil {
				return fmt.Errorf("unable to unmarshal json for global rule: %w", err)
			}

			r.GlobalRules = append(r.GlobalRules, globalRule)

		case tuf.GlobalRuleBlockForcePushesType:
			globalRule := &GlobalRuleBlockForcePushes{}
			if err := json.Unmarshal(globalRuleBytes, globalRule); err != nil {
				return fmt.Errorf("unable to unmarshal json for global rule: %w", err)
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

// addKey adds a key to the RootMetadata instance.
func (r *RootMetadata) addKey(key tuf.Principal) error {
	if r.Keys == nil {
		r.Keys = map[string]*Key{}
	}

	keyT, isKnownType := key.(*Key)
	if !isKnownType {
		return tuf.ErrInvalidPrincipalType
	}

	r.Keys[key.ID()] = keyT
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

type GlobalRuleThreshold struct {
	Name      string   `json:"name"`
	Type      string   `json:"type"`
	Paths     []string `json:"paths"`
	Threshold int      `json:"threshold"`
}

func NewGlobalRuleThreshold(name string, paths []string, threshold int) *GlobalRuleThreshold {
	return &GlobalRuleThreshold{
		Name:      name,
		Type:      tuf.GlobalRuleThresholdType,
		Paths:     paths,
		Threshold: threshold,
	}
}

func (g *GlobalRuleThreshold) GetName() string {
	return g.Name
}

func (g *GlobalRuleThreshold) Matches(path string) bool {
	for _, pattern := range g.Paths {
		// We validate pattern when it's added to / updated in the metadata
		if matches := fnmatch.Match(pattern, path, 0); matches {
			return true
		}
	}
	return false
}

func (g *GlobalRuleThreshold) GetProtectedNamespaces() []string {
	return g.Paths
}

func (g *GlobalRuleThreshold) GetThreshold() int {
	return g.Threshold
}

type GlobalRuleBlockForcePushes struct {
	Name  string   `json:"name"`
	Type  string   `json:"type"`
	Paths []string `json:"paths"`
}

func NewGlobalRuleBlockForcePushes(name string, paths []string) (*GlobalRuleBlockForcePushes, error) {
	for _, path := range paths {
		if !strings.HasPrefix(path, "git:") { // TODO: set prefix correctly
			return nil, tuf.ErrGlobalRuleBlockForcePushesOnlyAppliesToGitPaths
		}
	}
	return &GlobalRuleBlockForcePushes{
		Name:  name,
		Type:  tuf.GlobalRuleBlockForcePushesType,
		Paths: paths,
	}, nil
}

func (g *GlobalRuleBlockForcePushes) GetName() string {
	return g.Name
}

func (g *GlobalRuleBlockForcePushes) Matches(path string) bool {
	for _, pattern := range g.Paths {
		// We validate pattern when it's added to / updated in the metadata
		if matches := fnmatch.Match(pattern, path, 0); matches {
			return true
		}
	}
	return false
}

func (g *GlobalRuleBlockForcePushes) GetProtectedNamespaces() []string {
	return g.Paths
}

type PropagationDirective struct {
	Name                string `json:"name"`
	UpstreamRepository  string `json:"upstreamRepository"`
	UpstreamReference   string `json:"upstreamReference"`
	DownstreamReference string `json:"downstreamReference"`
	DownstreamPath      string `json:"downstreamPath"`
}

func (p *PropagationDirective) GetName() string {
	return p.Name
}

func (p *PropagationDirective) GetUpstreamRepository() string {
	return p.UpstreamRepository
}

func (p *PropagationDirective) GetUpstreamReference() string {
	return p.UpstreamReference
}

func (p *PropagationDirective) GetDownstreamReference() string {
	return p.DownstreamReference
}

func (p *PropagationDirective) GetDownstreamPath() string {
	return p.DownstreamPath
}

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
	Name                  string `json:"name"`
	Location              string `json:"location"`
	InitialRootPrincipals []*Key `json:"initialRootPrincipals"`
}

func (o *OtherRepository) GetName() string {
	return o.Name
}

func (o *OtherRepository) GetLocation() string {
	return o.Location
}

func (o *OtherRepository) GetInitialRootPrincipals() []tuf.Principal {
	initialRootPrincipals := []tuf.Principal{}
	for _, key := range o.InitialRootPrincipals {
		initialRootPrincipals = append(initialRootPrincipals, key)
	}
	return initialRootPrincipals
}

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

// Hook defines the schema for a hook.
type Hook struct {
	Name         string              `json:"name"`
	PrincipalIDs *set.Set[string]    `json:"principals"`
	Hashes       map[string]string   `json:"hashes"`
	Environment  tuf.HookEnvironment `json:"environment"`
	Modules      []string            `json:"modules"`
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
func (h *Hook) GetHashes() map[string]string {
	return h.Hashes
}

func (h *Hook) GetBlobID() gitinterface.Hash {
	hash, _ := gitinterface.NewHash(h.Hashes[gitinterface.GitBlobHashName])
	return hash
}

// GetEnvironment returns the environment that the hook is to run in.
func (h *Hook) GetEnvironment() tuf.HookEnvironment {
	return h.Environment
}

// GetModules returns the Lua modules that the hook will have access to.
func (h *Hook) GetModules() []string {
	return h.Modules
}
