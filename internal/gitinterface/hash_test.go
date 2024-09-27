// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"encoding/hex"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewHash(t *testing.T) {
	tests := map[string]struct {
		hash          string
		expectedError error
	}{
		"correctly encoded SHA-1 hash": {
			hash: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391",
		},
		"correctly encoded SHA-256 hash": {
			hash: "61658570165bc04af68cef20d72da49b070dc9d8cd7c8a526c950b658f4d3ccf",
		},
		"correctly encoded SHA-1 zero hash": {
			hash: "0000000000000000000000000000000000000000",
		},
		"correctly encoded SHA-256 zero hash": {
			hash: "0000000000000000000000000000000000000000000000000000000000000000",
		},
		"incorrect length SHA-1 hash": {
			hash:          "e69de29bb2d1d6434b8",
			expectedError: ErrInvalidHashLength,
		},
		"incorrect length SHA-256 hash": {
			hash:          "61658570165bc04af68cef20d72da49b070dc9d8cd7c8a526c950b658f4d3ccfabcdef",
			expectedError: ErrInvalidHashLength,
		},
		"incorrectly encoded SHA-1 hash": {
			hash:          "e69de29bb2d1d6434b8b29ae775ad8c2e48c539g", // last char is 'g'
			expectedError: ErrInvalidHashEncoding,
		},
		"incorrectly encoded SHA-256 hash": {
			hash:          "61658570165bc04af68cef20d72da49b070dc9d8cd7c8a526c950b658f4d3ccg", // last char is 'g'
			expectedError: ErrInvalidHashEncoding,
		},
	}

	for name, test := range tests {
		hash, err := NewHash(test.hash)
		if test.expectedError == nil {
			expectedHash, secErr := hex.DecodeString(test.hash)
			require.Nil(t, secErr)

			assert.Equal(t, Hash(expectedHash), hash)
			assert.Equal(t, test.hash, hash.String())
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		} else {
			assert.ErrorIs(t, err, test.expectedError, fmt.Sprintf("unexpected error in test '%s'", name))
		}
	}
}
