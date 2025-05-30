// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gpg

import (
	"context"
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
)

func TestGPG(t *testing.T) {
	// fingerprint should be the same among public/private keys
	fprGPG1 := "157507bbe151e378ce8126c1dcfe043cdd2db96e"
	fprGPG2 := "7707e87f10df498472babc32e517e211cb23a9e9"

	tests := []struct {
		keyName      string
		pubkeyBytes  []byte
		privkeyBytes []byte
		keyID        string
	}{
		{"gpg1", artifacts.GPGKey1Public, artifacts.GPGKey1Private, fprGPG1},
		{"gpg2", artifacts.GPGKey2Public, artifacts.GPGKey2Private, fprGPG2},
	}

	data := []byte("DATA")
	notData := []byte("NOT DATA")

	// Run tests
	for _, test := range tests {
		t.Run(test.keyName, func(t *testing.T) {
			// load publ key for verifier
			pubKey, err := LoadGPGKeyFromBytes(test.pubkeyBytes)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}
			assert.Equal(t, test.keyID, pubKey.KeyID)

			verifier, err := NewVerifierFromKey(pubKey)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}

			// load priv key for signer
			privKey, err := LoadGPGPrivKeyFromBytes(test.privkeyBytes)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}
			assert.Equal(t, test.keyID, privKey.KeyID)

			signer, err := NewSignerFromKey(privKey)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}

			sig, err := signer.Sign(context.Background(), data)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}

			err = verifier.Verify(context.Background(), data, sig)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}

			err = verifier.Verify(context.Background(), notData, sig)
			if err == nil {
				t.Fatalf("%s: %v", test.keyName, err)
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
