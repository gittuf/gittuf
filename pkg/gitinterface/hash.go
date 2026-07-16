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

// IsZero returns true if the hash denotes "no object": a nil / empty value or
// the zero hash for either SHA-1 or SHA-256.
func (h Hash) IsZero() bool {
	return len(h) == 0 || bytes.Equal(h[:], zeroSHA1HashBytes[:]) || bytes.Equal(h[:], zeroSHA256HashBytes[:])
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
		return nil, ErrInvalidHashLength
	}

	hash, err := hex.DecodeString(h)
	if err != nil {
		return nil, ErrInvalidHashEncoding
	}

	return Hash(hash), nil
}
