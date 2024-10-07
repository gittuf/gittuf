// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"time"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/tuf"
)

var (
	ErrRootMetadataNil    = errors.New("rootMetadata is nil")
	ErrTargetsMetadataNil = errors.New("targetsMetadata not found")
)

const GitHubAppRoleName = "github-app"

// InitializeRootMetadata initializes a new instance of tuf.RootMetadata with
// default values and a given key. The default values are version set to 1,
// expiry date set to one year from now, and the provided key is added.
func InitializeRootMetadata(key *tuf.Key) *tuf.RootMetadata {
	rootMetadata := tuf.NewRootMetadata()
	rootMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	rootMetadata.AddKey(key)

	rootMetadata.AddRole(RootRoleName, tuf.Role{
		KeyIDs:    set.NewSetFromItems(key.KeyID),
		Threshold: 1,
	})

	return rootMetadata
}

// AddRootKey adds rootKey as a trusted public key in rootMetadata for the
// Root role.
func AddRootKey(rootMetadata *tuf.RootMetadata, rootKey *tuf.Key) (*tuf.RootMetadata, error) {
	if rootMetadata == nil {
		return nil, ErrRootMetadataNil
	}

	err := rootMetadata.AddRootKey(rootKey)
	if err != nil {
		return nil, err
	}

	return rootMetadata, nil
}

// DeleteRootKey removes keyID from the list of trusted Root
// public keys in rootMetadata. It does not remove the key entry itself as it
// does not check if other roles can be verified using the same key.
func DeleteRootKey(rootMetadata *tuf.RootMetadata, keyID string) (*tuf.RootMetadata, error) {
	if rootMetadata == nil {
		return nil, ErrRootMetadataNil
	}

	if err := rootMetadata.DeleteRootKey(keyID); err != nil {
		return nil, err
	}

	return rootMetadata, nil
}

// AddTargetsKey adds the 'targetsKey' as a trusted public key in 'rootMetadata'
// for the top level Targets role.
func AddTargetsKey(rootMetadata *tuf.RootMetadata, targetsKey *tuf.Key) (*tuf.RootMetadata, error) {
	if rootMetadata == nil {
		return nil, ErrRootMetadataNil
	}

	if err := rootMetadata.AddTargetsKey(targetsKey); err != nil {
		return nil, err
	}

	return rootMetadata, nil
}

// DeleteTargetsKey removes the key matching 'keyID' from trusted public keys
// for top level Targets role in 'rootMetadata'. Note: It doesn't remove the key
// entry itself as it doesn't check if other roles can use the same key.
func DeleteTargetsKey(rootMetadata *tuf.RootMetadata, keyID string) (*tuf.RootMetadata, error) {
	if rootMetadata == nil {
		return nil, ErrRootMetadataNil
	}

	if err := rootMetadata.DeleteTargetsKey(keyID); err != nil {
		return nil, err
	}

	return rootMetadata, nil
}

// AddGitHubAppKey adds the 'appKey' as a trusted public key in 'rootMetadata'
// for the special GitHub app role. This key is used to verify GitHub pull
// request approval attestation signatures.
func AddGitHubAppKey(rootMetadata *tuf.RootMetadata, appKey *tuf.Key) (*tuf.RootMetadata, error) {
	if rootMetadata == nil {
		return nil, ErrRootMetadataNil
	}

	if err := rootMetadata.AddGitHubAppKey(appKey); err != nil {
		return nil, err
	}

	return rootMetadata, nil
}

// DeleteGitHubAppKey removes the special GitHub app role from the root
// metadata.
func DeleteGitHubAppKey(rootMetadata *tuf.RootMetadata) (*tuf.RootMetadata, error) {
	if rootMetadata == nil {
		return nil, ErrRootMetadataNil
	}

	rootMetadata.DeleteGitHubAppKey()
	return rootMetadata, nil
}

// EnableGitHubAppApprovals sets GitHubApprovalsTrusted to true in the
// root metadata.
func EnableGitHubAppApprovals(rootMetadata *tuf.RootMetadata) (*tuf.RootMetadata, error) {
	if rootMetadata == nil {
		return nil, ErrRootMetadataNil
	}

	rootMetadata.EnableGitHubAppApprovals()
	return rootMetadata, nil
}

// DisableGitHubAppApprovals sets GitHubApprovalsTrusted to false in the root
// metadata.
func DisableGitHubAppApprovals(rootMetadata *tuf.RootMetadata) (*tuf.RootMetadata, error) {
	if rootMetadata == nil {
		return nil, ErrRootMetadataNil
	}

	rootMetadata.DisableGitHubAppApprovals()
	return rootMetadata, nil
}

// UpdateRootThreshold sets the threshold for the Root role.
func UpdateRootThreshold(rootMetadata *tuf.RootMetadata, threshold int) (*tuf.RootMetadata, error) {
	if rootMetadata == nil {
		return nil, ErrRootMetadataNil
	}

	if err := rootMetadata.UpdateRootThreshold(threshold); err != nil {
		return nil, err
	}

	return rootMetadata, nil
}

// UpdateTargetsThreshold sets the threshold for the top level Targets role.
func UpdateTargetsThreshold(rootMetadata *tuf.RootMetadata, threshold int) (*tuf.RootMetadata, error) {
	if rootMetadata == nil {
		return nil, ErrRootMetadataNil
	}

	if err := rootMetadata.UpdateTargetsThreshold(threshold); err != nil {
		return nil, err
	}

	return rootMetadata, nil
}
