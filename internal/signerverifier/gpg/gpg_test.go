// SPDX-License-Identifier: Apache-2.0

package gpg

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/stretchr/testify/assert"
)

func TestLoadGPGKeyFromBytes(t *testing.T) {
	keyBytes, err := os.ReadFile(filepath.Join("test-data", "gpg-pubkey.asc"))
	if err != nil {
		t.Fatal(err)
	}

	key, err := LoadGPGKeyFromBytes(keyBytes)
	assert.Nil(t, err)
	assert.Equal(t, signerverifier.GPGKeyType, key.KeyType)
	assert.Equal(t, signerverifier.GPGKeyType, key.Scheme)
	assert.Equal(t, "157507bbe151e378ce8126c1dcfe043cdd2db96e", key.KeyID)
}
