// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package loadpolicy

import (
	"crypto"
	"crypto/ed25519"
	"testing"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWithEntry(t *testing.T) {
	t.Parallel()

	entryID, err := githash.NewHash("0000000000000000000000000000000000000001")
	require.NoError(t, err)

	options := &Options{}
	WithEntry(entryID)(options)

	assert.Equal(t, entryID, options.EntryID)
	assert.True(t, options.AtEntry)
}

func TestWithInitialRootKeys(t *testing.T) {
	t.Parallel()

	key := ed25519.PublicKey(make([]byte, ed25519.PublicKeySize))

	options := &Options{}
	WithInitialRootKeys(key)(options)

	assert.Equal(t, []crypto.PublicKey{key}, options.InitialRootKeys)
}
