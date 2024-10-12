// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

// This package defines gittuf's take on TUF metadata. There are some minor
// changes, such as the addition of `custom` to delegation entries. Some of it,
// however, is inspired by or cloned from the go-tuf implementation.

import (
	"errors"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

var ErrTargetsNotEmpty = errors.New("`targets` field in gittuf Targets metadata must be empty")

// Key defines the structure for how public keys are stored in TUF metadata. It
// implements the tuf.Principal and is used for backwards compatibility where a
// Principal is always represented directly by a signing key or identity.
type Key signerverifier.SSLibKey

// NewKeyFromSSLibKey converts the signerverifier.SSLibKey into a Key object.
func NewKeyFromSSLibKey(key *signerverifier.SSLibKey) *Key {
	k := Key(*key)
	return &k
}

// ID implements the key's identifier. It implements the Principal interface.
func (k *Key) ID() string {
	return k.KeyID
}

// Keys returns the set of keys (using the signerverifier.SSLibKey definition)
// associated with the principal.
func (k *Key) Keys() []*signerverifier.SSLibKey {
	key := signerverifier.SSLibKey(*k)
	return []*signerverifier.SSLibKey{&key}
}

// Role records common characteristics recorded in a role entry in Root metadata
// and in a delegation entry.
type Role struct {
	KeyIDs    *set.Set[string] `json:"keyids"`
	Threshold int              `json:"threshold"`
}
