// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

var (
	ErrInvalidHashEncoding = githash.ErrInvalidHashEncoding
	ErrInvalidHashLength   = githash.ErrInvalidHashLength

	// ErrReferenceNotFound aliases the gitstore sentinel that any
	// gitstore.Storer returns when a reference does not exist.
	ErrReferenceNotFound = gitstore.ErrReferenceNotFound
)

// NewHash returns a githash.Hash from a hex encoded SHA-1 or SHA-256 string.
func NewHash(h string) (githash.Hash, error) {
	return githash.NewHash(h)
}
