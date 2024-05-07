// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"encoding/hex"
	"errors"
)

const (
	zeroSHA1HashString   = "0000000000000000000000000000000000000000"
	zeroSHA256HashString = "0000000000000000000000000000000000000000000000000000000000000000"
)

var (
	ErrInvalidHashEncoding = errors.New("hash string is not hex encoded")
	ErrInvalidHashLength   = errors.New("hash string is wrong length")
)

type Hash struct {
	hash string
}

func (h Hash) String() string {
	return h.hash
}

func (h Hash) IsZero() bool {
	return h == ZeroHash
}

var ZeroHash = Hash{hash: zeroSHA1HashString}

func NewHash(h string) (Hash, error) {
	_, err := hex.DecodeString(h)
	if err != nil {
		return ZeroHash, ErrInvalidHashEncoding
	}

	if len(h) != len(zeroSHA1HashString) && len(h) != len(zeroSHA256HashString) {
		return ZeroHash, ErrInvalidHashLength
	}

	return Hash{hash: h}, nil
}
