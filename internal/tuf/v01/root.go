// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/danwakefield/fnmatch"
	"github.com/gittuf/gittuf/internal/common/set"
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

func (r *RootMetadata) UnmarshalJSON(data []byte) error {
	// this type _has_ to be a copy of RootMetadata, minus the use of
	// json.RawMessage for tuf interfaces
	type tempType struct {
		Type                   string            `json:"type"`
		Expires                string            `json:"expires"`
		RepositoryLocation     string            `json:"repositoryLocation,omitempty"`
		Keys                   map[string]*Key   `json:"keys"`
		Roles                  map[string]Role   `json:"roles"`
		GitHubApprovalsTrusted bool              `json:"githubApprovalsTrusted"`
		GlobalRules            []json.RawMessage `json:"globalRules,omitempty"`
		Propagations           []json.RawMessage `json:"propagations,omitempty"`
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
