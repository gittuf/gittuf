// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"encoding/json"
	"fmt"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/tuf"
)

const (
	RootVersion = "https://gittuf.dev/policy/root/v0.2"
)

// RootMetadata defines the schema of TUF's Root role.
type RootMetadata struct {
	Type                   string                   `json:"type"`
	Version                string                   `json:"schemaVersion"`
	Expires                string                   `json:"expires"`
	Principals             map[string]tuf.Principal `json:"principals"`
	Roles                  map[string]Role          `json:"roles"`
	GitHubApprovalsTrusted bool                     `json:"githubApprovalsTrusted"`
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
func (r *RootMetadata) AddGitHubAppPrincipal(principal tuf.Principal) error {
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
	r.addRole(tuf.GitHubAppRoleName, role) // AddRole replaces the specified role if it already exists
	return nil
}

// DeleteGitHubAppPrincipal removes the special GitHub app role from the root
// metadata.
func (r *RootMetadata) DeleteGitHubAppPrincipal() {
	// TODO: support multiple principals / threshold for app
	delete(r.Roles, tuf.GitHubAppRoleName)
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
	// json.RawMessage in place of tuf.Principal
	type tempType struct {
		Type                   string                     `json:"type"`
		Version                string                     `json:"schemaVersion"`
		Expires                string                     `json:"expires"`
		Principals             map[string]json.RawMessage `json:"principals"`
		Roles                  map[string]Role            `json:"roles"`
		GitHubApprovalsTrusted bool                       `json:"githubApprovalsTrusted"`
	}

	temp := &tempType{}
	if err := json.Unmarshal(data, &temp); err != nil {
		return fmt.Errorf("unable to unmarshal json: %w", err)
	}

	r.Type = temp.Type
	r.Version = temp.Version
	r.Expires = temp.Expires

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

	return nil
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
