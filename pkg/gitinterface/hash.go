// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"encoding/hex"
	"errors"
)

const (
	GitBlobHashName = "gitBlob"
	SHA256HashName  = "sha256"
)

var (
	zeroSHA1HashBytes   = [sha1.Size]byte{}
	zeroSHA256HashBytes = [sha256.Size]byte{}
)

var (
	ErrInvalidHashEncoding = errors.New("hash string is not hex encoded")
	ErrInvalidHashLength   = errors.New("hash string is wrong length")
)

// Hash represents a Git object hash. It is a lightweight wrapper around the
// standard hex encoded representation of a SHA-1 or SHA-256 hash used by Git.

type Hash []byte

// String returns the hex encoded hash.
func (h Hash) String() string {
	return hex.EncodeToString(h[:])
}

// IsZero compares the hash to see if it's the zero hash for either SHA-1 or
// SHA-256.
func (h Hash) IsZero() bool {
	return bytes.Equal(h[:], zeroSHA1HashBytes[:]) || bytes.Equal(h[:], zeroSHA256HashBytes[:])
}

// Equal compares the hash to another provided Hash to see if they're equal.
func (h Hash) Equal(other Hash) bool {
	return bytes.Equal(h[:], other[:])
}

// IsSHA256 returns true if the hash is a SHA-256 Git object ID, determined by
// its length. SHA-1 object IDs are shorter.
func (h Hash) IsSHA256() bool {
	return len(h) == sha256.Size
}

// ZeroHash represents an empty SHA-1 Hash. It is safe to use as an
// error-return sentinel and in comparisons via Hash.IsZero (which matches both
// SHA-1 and SHA-256 zero hashes). When the value is passed to Git or compared
// against a Git-produced ID, prefer Repository.ZeroHash, which returns the zero
// hash matching the repository's object format.
var ZeroHash = Hash(zeroSHA1HashBytes[:])

// ZeroHash returns the all-zeroes Hash for the repository's object format. Git
// uses this value to denote, for example, the absence of a previous value when
// creating or deleting a reference.
func (r *Repository) ZeroHash() Hash {
	if r.objectFormat == ObjectFormatSHA256 {
		return Hash(zeroSHA256HashBytes[:])
	}
	return Hash(zeroSHA1HashBytes[:])
}

// NewHash returns a Hash object after ensuring the input string is correctly
// encoded.
func NewHash(h string) (Hash, error) {
	if len(h) != (sha1.Size*2) && len(h) != (sha256.Size*2) {
		return ZeroHash, ErrInvalidHashLength
	}

	hash, err := hex.DecodeString(h)
	if err != nil {
		return ZeroHash, ErrInvalidHashEncoding
	}

	return Hash(hash), nil
}
