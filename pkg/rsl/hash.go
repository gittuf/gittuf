// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

// Hash is the Git object hash the RSL operates over, aliased from
// githash.Hash (and therefore identical to gitinterface.Hash).
type Hash = githash.Hash

var (
	ErrInvalidHashEncoding = githash.ErrInvalidHashEncoding
	ErrInvalidHashLength   = githash.ErrInvalidHashLength

	// ErrReferenceNotFound aliases the gitstore sentinel that any
	// gitstore.Storer returns when a reference does not exist.
	ErrReferenceNotFound = gitstore.ErrReferenceNotFound
)

// NewHash returns a Hash from a hex encoded SHA-1 or SHA-256 string.
func NewHash(h string) (Hash, error) {
	return githash.NewHash(h)
}
