// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gpg

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
)

func TestGPG(t *testing.T) {
	// Q: Is using the primary key's fingerprint appropriate?
	fprGPG1 := "157507bbe151e378ce8126c1dcfe043cdd2db96e"
	// fprGPG1Pub := "157507bbe151e378ce8126c1dcfe043cdd2db96e"
	fprGPG2 := "7707e87f10df498472babc32e517e211cb23a9e9"

	tests := []struct {
		keyName  string
		keyBytes []byte
		keyID    string
	}{
		// Q: For keyid, use fingerprint?
		{"gpg1", artifacts.GPGKey1Private, fprGPG1},
		// {"gpg1.pub", artifacts.GPGKey1Public, fprGPG1},
		{"gpg2", artifacts.GPGKey2Private, fprGPG2},
		// {"gpg2.pub", artifacts.GPGKey2Public, fprGPG2},
	}
	// Setup tests
	tmpDir := t.TempDir()
	// Write script to mock password prompt

	scriptPath := filepath.Join(tmpDir, "askpass.sh")
	if err := os.WriteFile(scriptPath, artifacts.AskpassScript, 0o500); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	// Write test key pairs to temp dir with permissions required by ssh-keygen
	for _, test := range tests {
		keyPath := filepath.Join(tmpDir, test.keyName)
		if err := os.WriteFile(keyPath, test.keyBytes, 0o600); err != nil {
			t.Fatal(err)
		}
	}

	data := []byte("DATA")
	notData := []byte("NOT DATA")

	// Run tests
	for _, test := range tests {
		t.Run(test.keyName, func(t *testing.T) {
			if strings.Contains(test.keyName, "_enc") {
				if runtime.GOOS == "windows" {
					t.Skip("TODO: test encrypted keys on windows")
				}
				// Q: what are these?
				t.Setenv("SSH_ASKPASS", scriptPath)
				t.Setenv("SSH_ASKPASS_REQUIRE", "force")
			}

			keyPath := filepath.Join(tmpDir, test.keyName)

			key, err := NewKeyFromFile(keyPath)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}
			assert.Equal(t,
				key.KeyID,
				test.keyID,
			)

			verifier, err := NewVerifierFromKey(key)
			if err != nil {
				t.Fatalf("%s: %v", test.keyName, err)
			}

			signer, err := NewSignerFromFile(keyPath)
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
