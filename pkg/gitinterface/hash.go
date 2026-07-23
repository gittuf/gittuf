// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"github.com/gittuf/gittuf/pkg/githash"
)

const (
	GitBlobHashName = "gitBlob"
	SHA256HashName  = "sha256"
)

var (
	ErrInvalidHashEncoding = githash.ErrInvalidHashEncoding
	ErrInvalidHashLength   = githash.ErrInvalidHashLength
)

// Hash represents a Git object hash. It aliases githash.Hash so gittuf's
// dependency-light packages (such as rsl) can name the same concrete type
// without importing gitinterface and its signing/attestation stack.
type Hash = githash.Hash

// ZeroHash represents an empty SHA-1 Hash. It is safe to use as an
// error-return sentinel and in comparisons via Hash.IsZero (which matches both
// SHA-1 and SHA-256 zero hashes). When the value is passed to Git or compared
// against a Git-produced ID, prefer Repository.ZeroHash, which returns the zero
// hash matching the repository's object format.
var ZeroHash = githash.ZeroHash

// ZeroHash returns the all-zeroes Hash for the repository's object format. Git
// uses this value to denote, for example, the absence of a previous value when
// creating or deleting a reference.
func (r *Repository) ZeroHash() Hash {
	if r.objectFormat == ObjectFormatSHA256 {
		return githash.ZeroHashSHA256
	}
	return githash.ZeroHash
}

// NewHash returns a Hash object after ensuring the input string is correctly
// encoded.
func NewHash(h string) (Hash, error) {
	return githash.NewHash(h)
}
