package policy

import (
	"errors"
	"time"

	"github.com/adityasaky/gittuf/internal/tuf"
)

var ErrCannotMeetThreshold = errors.New("removing key will drop authorized keys below threshold")

// InitializeRootMetadata creates a new instance of tuf.RootMetadata with
// default values.
func InitializeRootMetadata(key *tuf.Key) *tuf.RootMetadata {
	rootMetadata := tuf.NewRootMetadata()
	rootMetadata.SetVersion(1)
	rootMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	rootMetadata.AddKey(*key)

	rootMetadata.AddRole(RootRoleName, tuf.Role{
		KeyIDs:    []string{key.KeyID},
		Threshold: 1,
	})

	return rootMetadata
}

// AddTargetsKey adds targetsKey as a trusted public key in rootMetadata for the
// top level Targets role.
func AddTargetsKey(rootMetadata *tuf.RootMetadata, targetsKey *tuf.Key) *tuf.RootMetadata {
	rootMetadata.Keys[targetsKey.KeyID] = *targetsKey
	if _, ok := rootMetadata.Roles[TargetsRoleName]; !ok {
		rootMetadata.AddRole(TargetsRoleName, tuf.Role{
			KeyIDs:    []string{targetsKey.KeyID},
			Threshold: 1,
		})
		return rootMetadata
	}

	targetsRole := rootMetadata.Roles[TargetsRoleName]
	for _, keyID := range targetsRole.KeyIDs {
		if keyID == targetsKey.KeyID {
			return rootMetadata
		}
	}

	targetsRole.KeyIDs = append(targetsRole.KeyIDs, targetsKey.KeyID)
	rootMetadata.Roles[TargetsRoleName] = targetsRole

	return rootMetadata
}

// DeleteTargetsKey removes keyID from the list of trusted top level Targets
// public keys in rootMetadata. It does not remove the key entry itself as it
// does not check if other roles can be verified using the same key.
func DeleteTargetsKey(rootMetadata *tuf.RootMetadata, keyID string) (*tuf.RootMetadata, error) {
	if _, ok := rootMetadata.Roles[TargetsRoleName]; !ok {
		return rootMetadata, nil
	}

	targetsRole := rootMetadata.Roles[TargetsRoleName]

	if len(targetsRole.KeyIDs) <= targetsRole.Threshold {
		return nil, ErrCannotMeetThreshold
	}
	for i, k := range targetsRole.KeyIDs {
		if k == keyID {
			targetsRole.KeyIDs = append(targetsRole.KeyIDs[:i], targetsRole.KeyIDs[i+1:]...)
			break
		}
	}
	rootMetadata.Roles[TargetsRoleName] = targetsRole

	return rootMetadata, nil
}
