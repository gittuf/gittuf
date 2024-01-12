// SPDX-License-Identifier: Apache-2.0

package gpg

import (
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
)

func TestLoadGPGKeyFromBytes(t *testing.T) {
	keyBytes := artifacts.GPGKey1Public

	key, err := LoadGPGKeyFromBytes(keyBytes)
	assert.Nil(t, err)
	assert.Equal(t, signerverifier.GPGKeyType, key.KeyType)
	assert.Equal(t, signerverifier.GPGKeyType, key.Scheme)
	assert.Equal(t, "157507bbe151e378ce8126c1dcfe043cdd2db96e", key.KeyID)
}
