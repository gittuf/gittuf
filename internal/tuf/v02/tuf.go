// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

// This package defines gittuf's take on TUF metadata. There are some minor
// changes, such as the addition of `custom` to delegation entries. Some of it,
// however, is inspired by or cloned from the go-tuf implementation.

import (
	"fmt"
	"os"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	v01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

const (
	AllowV02MetadataKey = "GITTUF_ALLOW_V02_POLICY"

	associatedIdentityKey = "(associated identity)"
)

// AllowV02Metadata returns true if gittuf is in developer mode and
// GITTUF_ALLOW_V02_POLICY=1.
func AllowV02Metadata() bool {
	return dev.InDevMode() && os.Getenv(AllowV02MetadataKey) == "1"
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

type Person struct {
	PersonID             string            `json:"personID"`
	PublicKeys           map[string]*Key   `json:"keys"`
	AssociatedIdentities map[string]string `json:"associatedIdentities"`
	Custom               map[string]string `json:"custom"`
}

func (p *Person) ID() string {
	return p.PersonID
}

func (p *Person) Keys() []*signerverifier.SSLibKey {
	keys := make([]*signerverifier.SSLibKey, 0, len(p.PublicKeys))
	for _, key := range p.PublicKeys {
		key := signerverifier.SSLibKey(*key)
		keys = append(keys, &key)
	}

	return keys
}

func (p *Person) CustomMetadata() map[string]string {
	var metadata map[string]string

	for provider, identity := range p.AssociatedIdentities {
		if metadata == nil {
			metadata = map[string]string{}
		}
		metadata[fmt.Sprintf("%s %s", associatedIdentityKey, provider)] = identity
	}

	for key, value := range p.Custom {
		if metadata == nil {
			metadata = map[string]string{}
		}
		metadata[key] = value
	}

	return metadata
}

// Role records common characteristics recorded in a role entry in Root metadata
// and in a delegation entry.
type Role struct {
	PrincipalIDs *set.Set[string] `json:"principalIDs"`
	Threshold    int              `json:"threshold"`
}
