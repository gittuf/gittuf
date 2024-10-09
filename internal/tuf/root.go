// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tuf

import "github.com/gittuf/gittuf/internal/common/set"

const GitHubAppRoleName = "github-app"

// RootMetadata defines the schema of TUF's Root role.
type RootMetadata struct {
	Type                   string          `json:"type"`
	Expires                string          `json:"expires"`
	Keys                   map[string]*Key `json:"keys"`
	Roles                  map[string]Role `json:"roles"`
	GitHubApprovalsTrusted bool            `json:"githubApprovalsTrusted"`
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

// AddKey adds a key to the RootMetadata instance.
func (r *RootMetadata) AddKey(key *Key) {
	if r.Keys == nil {
		r.Keys = map[string]*Key{}
	}

	r.Keys[key.KeyID] = key
}

// AddRole adds a role object and associates it with roleName in the
// RootMetadata instance.
func (r *RootMetadata) AddRole(roleName string, role Role) {
	if r.Roles == nil {
		r.Roles = map[string]Role{}
	}

	r.Roles[roleName] = role
}

// AddRootKey adds the specified key to the root metadata and authorizes the key
// for the root role.
func (r *RootMetadata) AddRootKey(key *Key) error {
	if key == nil {
		return ErrRootKeyNil
	}

	// Add key to metadata
	r.AddKey(key)

	if _, ok := r.Roles[RootRoleName]; !ok {
		// Create a new root role entry with this key
		r.AddRole(RootRoleName, Role{
			KeyIDs:    set.NewSetFromItems(key.KeyID),
			Threshold: 1,
		})

		return nil
	}

	// Add key ID to the root role if it's not already in it
	rootRole := r.Roles[RootRoleName]
	rootRole.KeyIDs.Add(key.KeyID)
	r.Roles[RootRoleName] = rootRole
	return nil
}

// DeleteRootKey removes keyID from the list of trusted Root
// public keys in rootMetadata. It does not remove the key entry itself as it
// does not check if other roles can be verified using the same key.
func (r *RootMetadata) DeleteRootKey(keyID string) error {
	if _, ok := r.Roles[RootRoleName]; !ok {
		return nil
	}

	rootRole := r.Roles[RootRoleName]
	if rootRole.KeyIDs.Len() <= rootRole.Threshold {
		return ErrCannotMeetThreshold
	}

	rootRole.KeyIDs.Remove(keyID)
	r.Roles[RootRoleName] = rootRole
	return nil
}

// AddTargetsKey adds the 'targetsKey' as a trusted public key in 'rootMetadata'
// for the top level Targets role.
func (r *RootMetadata) AddTargetsKey(key *Key) error {
	if key == nil {
		return ErrTargetsKeyNil
	}

	// Add key to the metadata file
	r.AddKey(key)

	if _, ok := r.Roles[TargetsRoleName]; !ok {
		// Create a new targets role entry with this key
		r.AddRole(TargetsRoleName, Role{
			KeyIDs:    set.NewSetFromItems(key.KeyID),
			Threshold: 1,
		})

		return nil
	}

	targetsRole := r.Roles[TargetsRoleName]
	targetsRole.KeyIDs.Add(key.KeyID)
	r.Roles[TargetsRoleName] = targetsRole

	return nil
}

// DeleteTargetsKey removes the key matching 'keyID' from trusted public keys
// for top level Targets role in 'rootMetadata'. Note: It doesn't remove the key
// entry itself as it doesn't check if other roles can use the same key.
func (r *RootMetadata) DeleteTargetsKey(keyID string) error {
	if keyID == "" {
		return ErrKeyIDEmpty
	}

	if _, ok := r.Roles[TargetsRoleName]; !ok {
		return nil
	}

	targetsRole := r.Roles[TargetsRoleName]

	if targetsRole.KeyIDs.Len() <= targetsRole.Threshold {
		return ErrCannotMeetThreshold
	}

	targetsRole.KeyIDs.Remove(keyID)
	r.Roles[TargetsRoleName] = targetsRole
	return nil
}

// AddGitHubAppKey adds the 'appKey' as a trusted public key in 'rootMetadata'
// for the special GitHub app role. This key is used to verify GitHub pull
// request approval attestation signatures.
func (r *RootMetadata) AddGitHubAppKey(key *Key) error {
	if key == nil {
		return ErrGitHubAppKeyNil
	}

	// TODO: support multiple keys / threshold for app
	r.AddKey(key)
	role := Role{
		KeyIDs:    set.NewSetFromItems(key.KeyID),
		Threshold: 1,
	}
	r.AddRole(GitHubAppRoleName, role) // AddRole replaces the specified role if it already exists
	return nil
}

// DeleteGitHubAppKey removes the special GitHub app role from the root
// metadata.
func (r *RootMetadata) DeleteGitHubAppKey() {
	// TODO: support multiple keys / threshold for app
	delete(r.Roles, GitHubAppRoleName)
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
	rootRole, ok := r.Roles[RootRoleName]
	if !ok {
		return ErrRootMetadataNil
	}

	if rootRole.KeyIDs.Len() < threshold {
		return ErrCannotMeetThreshold
	}
	rootRole.Threshold = threshold
	r.Roles[RootRoleName] = rootRole
	return nil
}

// UpdateTargetsThreshold sets the threshold for the top level Targets role.
func (r *RootMetadata) UpdateTargetsThreshold(threshold int) error {
	targetsRole, ok := r.Roles[TargetsRoleName]
	if !ok {
		return ErrTargetsMetadataNil
	}

	if targetsRole.KeyIDs.Len() < threshold {
		return ErrCannotMeetThreshold
	}
	targetsRole.Threshold = threshold
	r.Roles[TargetsRoleName] = targetsRole
	return nil
}
