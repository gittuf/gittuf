// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gpg

import (
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
)

func TestGPG(t *testing.T) {
	// Make a test GPG keyring in tempdir to use for tests
	SetupTestGPGHomeDir(t, artifacts.GPGKey1Private, artifacts.GPGKey2Private)

	// Test GPG key fingerprints
	fingerprintGPG1 := "157507bbe151e378ce8126c1dcfe043cdd2db96e"
	fingerprintGPG2 := "7707e87f10df498472babc32e517e211cb23a9e9"

	tests := []struct {
		pubkeyBytes []byte
		keyID       string
	}{
		{
			pubkeyBytes: artifacts.GPGKey1Public,
			keyID:       fingerprintGPG1,
		},
		{
			pubkeyBytes: artifacts.GPGKey2Public,
			keyID:       fingerprintGPG2,
		},
	}

	data := []byte("DATA")
	notData := []byte("NOT DATA")

	// Run tests
	for _, test := range tests {
		t.Run(test.keyID, func(t *testing.T) {
			// load publ key for verifier
			pubKey, err := LoadGPGKeyFromBytes(test.pubkeyBytes)
			if err != nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}
			assert.Equal(t, test.keyID, pubKey.KeyID)

			verifier, err := NewVerifierFromKey(pubKey)
			if err != nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}

			signer, err := NewSignerFromKeyID(test.keyID)
			if err != nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}

			sig, err := signer.Sign(t.Context(), data)
			if err != nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}

			err = verifier.Verify(t.Context(), data, sig)
			if err != nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}

			err = verifier.Verify(t.Context(), notData, sig)
			if err == nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}
		})
	}
}

func TestLoadGPGKeyFromBytes(t *testing.T) {
	keyBytes := artifacts.GPGKey1Public

	key, err := LoadGPGKeyFromBytes(keyBytes)
	assert.Nil(t, err)
	assert.Equal(t, KeyType, key.KeyType)
	assert.Equal(t, KeyType, key.Scheme)
	assert.Equal(t, "157507bbe151e378ce8126c1dcfe043cdd2db96e", key.KeyID)
}
