// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v03

import (
	"os"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	v01 "github.com/gittuf/gittuf/internal/tuf/v01"
	v02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

const AllowV03MetadataKey = "GITTUF_ALLOW_V03_POLICY"

// AllowV03Metadata returns true if gittuf is in developer mode and
// GITTUF_ALLOW_V03_POLICY = 1.
func AllowV03Metadata() bool {
	return dev.InDevMode() && os.Getenv(AllowV03MetadataKey) == "1"
}

// Key defines the structure for how public keys are stored in TUF metadata. It
// implements the tuf.Principal and is used for backwards compatibility where a
// Principal is always represented directly by a signing key or identity.
type Key = v01.Key

// NewKeyFromSSLibKey converts the signerverifier.SSLibKey into a Key object.
func NewKeyFromSSLibKey(key *signerverifier.SSLibKey) *Key {
	k := Key(*key)
	return &k
}

// Person defines the structure for how a person identity is stored in policy
// metadata. It implements tuf.Principal.
type Person = v02.Person

// Team defines the structure for how a team identity is stored in policy
// metadata. It implements tuf.Principal.
type Team struct {
	// TeamID is a unique name or identifier for a team.
	TeamID string `json:"teamID"`
	// PublicKeys stores all public keys for all members of a team.
	PublicKeys []*Key `json:"keys"`
	// Metadata stores custom metadata for a team.
	Metadata map[string]string `json:"custom"`
	// Members stores references to individual persons of a team.
	Members []*Person `json:"members"`
	// Threshold defines the minimum number required for a team to reach
	// agreement.
	Threshold int `json:"threshold"`
}

// ID returns the team ID of a team.
func (t *Team) ID() string {
	return t.TeamID
}

// Keys returns all keys of a team.
func (t *Team) Keys() []*signerverifier.SSLibKey {
	keys := make([]*signerverifier.SSLibKey, 0, len(t.PublicKeys))
	for _, key := range t.PublicKeys {
		key := signerverifier.SSLibKey(*key)
		keys = append(keys, &key)
	}

	return keys
}

// CustomMetadata returns the custom metadata of a team.
func (t *Team) CustomMetadata() map[string]string {
	return t.Metadata
}

func (t *Team) GetMembers() []*Person {
	return t.Members
}

func (t *Team) GetThreshold() int {
	return t.Threshold
}

// Role records common characteristics recorded in a role entry in Root metadata
// and in a delegation entry.
type Role struct {
	PrincipalIDs *set.Set[string] `json:"principalIDs"`
	Threshold    int              `json:"threshold"`
}
