// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package githash

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashMethods(t *testing.T) {
	t.Parallel()

	sha1Hex := "abcdef12345678900987654321fedcbaabcdef12"
	sha256Hex := "abcdef12345678900987654321fedcbaabcdef12345678900987654321fedcba"

	sha1Hash, err := NewHash(sha1Hex)
	require.Nil(t, err)

	sha256Hash, err := NewHash(sha256Hex)
	require.Nil(t, err)

	t.Run("String round-trips SHA-1", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, sha1Hex, sha1Hash.String())
	})

	t.Run("String round-trips SHA-256", func(t *testing.T) {
		t.Parallel()
		assert.Equal(t, sha256Hex, sha256Hash.String())
	})

	t.Run("Bytes length SHA-1", func(t *testing.T) {
		t.Parallel()
		b := sha1Hash.Bytes()
		assert.Len(t, b, 20)
	})

	t.Run("Bytes length SHA-256", func(t *testing.T) {
		t.Parallel()
		b := sha256Hash.Bytes()
		assert.Len(t, b, 32)
	})

	t.Run("IsSHA256 false for SHA-1", func(t *testing.T) {
		t.Parallel()
		assert.False(t, sha1Hash.IsSHA256())
	})

	t.Run("IsSHA256 true for SHA-256", func(t *testing.T) {
		t.Parallel()
		assert.True(t, sha256Hash.IsSHA256())
	})

	t.Run("Equal with own Bytes", func(t *testing.T) {
		t.Parallel()
		assert.True(t, sha1Hash.Equal(sha1Hash.Bytes()))
	})

	t.Run("Equal with different hash Bytes", func(t *testing.T) {
		t.Parallel()
		assert.False(t, sha1Hash.Equal(sha256Hash.Bytes()))
	})

	t.Run("Equal with another Hash directly", func(t *testing.T) {
		t.Parallel()
		// Hash is []byte, so another Hash is assignable to []byte.
		assert.True(t, sha1Hash.Equal([]byte(sha1Hash)))
		assert.False(t, sha1Hash.Equal([]byte(sha256Hash)))
	})
}

func TestIsZero(t *testing.T) {
	t.Parallel()

	nonZeroSHA1, err := NewHash("abcdef12345678900987654321fedcbaabcdef12")
	require.Nil(t, err)
	nonZeroSHA256, err := NewHash("abcdef12345678900987654321fedcbaabcdef12345678900987654321fedcba")
	require.Nil(t, err)

	tests := map[string]struct {
		hash     Hash
		expected bool
	}{
		"nil":              {hash: nil, expected: true},
		"empty":            {hash: Hash{}, expected: true},
		"SHA-1 zero":       {hash: ZeroHash, expected: true},
		"SHA-256 zero":     {hash: ZeroHashSHA256, expected: true},
		"SHA-1 non-zero":   {hash: nonZeroSHA1, expected: false},
		"SHA-256 non-zero": {hash: nonZeroSHA256, expected: false},
		"short non-zero":   {hash: Hash{0x01}, expected: false},
		"single zero byte": {hash: Hash{0x00}, expected: false},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.Equal(t, test.expected, test.hash.IsZero())
		})
	}
}
