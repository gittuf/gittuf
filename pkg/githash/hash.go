// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Package githash holds the concrete Git object hash type shared across gittuf.
// It depends only on the standard library, so packages that need to name a Git
// hash (gitinterface, rsl, ...) can do so without pulling in gitinterface's
// signing/attestation stack. gitinterface.Hash is an alias of githash.Hash, so
// the two are the same type everywhere.
package githash

import (
	"bytes"
	"crypto/sha1" //nolint:gosec
	"crypto/sha256"
	"encoding/hex"
	"errors"
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

// IsZero reports whether the hash is unset: nil, empty, or the zero hash of
// either the SHA-1 or SHA-256 object format.
func (h Hash) IsZero() bool {
	if len(h) == 0 {
		return true
	}

	return bytes.Equal(h[:], zeroSHA1HashBytes[:]) || bytes.Equal(h[:], zeroSHA256HashBytes[:])
}

// Equal compares the hash to the raw bytes of another hash to see if they're
// equal. It takes []byte so callers can compare against raw hash bytes
// without a conversion, since a Hash is assignable to []byte.
func (h Hash) Equal(other []byte) bool {
	return bytes.Equal(h, other)
}

// IsSHA256 returns true if the hash is a SHA-256 Git object ID, determined by
// its length. SHA-1 object IDs are shorter.
func (h Hash) IsSHA256() bool {
	return len(h) == sha256.Size
}

// Bytes returns the raw bytes of the hash.
func (h Hash) Bytes() []byte {
	return h[:]
}

// ZeroHash represents an empty SHA-1 Hash. It is safe to use as an
// error-return sentinel and in comparisons via Hash.IsZero (which matches nil
// and empty hashes as well as both SHA-1 and SHA-256 zero hashes). When the
// value is passed to Git or compared against a Git-produced ID, prefer
// gitinterface's Repository.ZeroHash, which returns the zero hash matching
// the repository's object format.
var ZeroHash = Hash(zeroSHA1HashBytes[:])

// ZeroHashSHA256 represents an empty SHA-256 Hash. As with ZeroHash, prefer
// gitinterface's Repository.ZeroHash when the repository's object format is
// known.
var ZeroHashSHA256 = Hash(zeroSHA256HashBytes[:])

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
