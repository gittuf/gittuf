// SPDX-License-Identifier: Apache-2.0

package common

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
)

func TestLoadSigner(t *testing.T) {
	tmpDir := t.TempDir()
	tests := map[string]struct {
		keyBytes       []byte
		publicKeyBytes []byte
	}{
		"ssh-rsa-key":     {keyBytes: artifacts.SSHRSAPrivate, publicKeyBytes: artifacts.SSHRSAPublicSSH},
		"ssh-ecdsa-key":   {keyBytes: artifacts.SSHECDSAPrivate, publicKeyBytes: artifacts.SSHECDSAPublicSSH},
		"ssh-ed25519-key": {keyBytes: artifacts.SSHED25519Private, publicKeyBytes: artifacts.SSHED25519PublicSSH},
	}

	for name, test := range tests {
		keyPath := filepath.Join(tmpDir, name)
		if err := os.WriteFile(keyPath, test.keyBytes, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", test.publicKeyBytes, 0o600); err != nil {
			t.Fatal(err)
		}

		signer, err := LoadSigner(keyPath)
		assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))

		_, err = signer.Sign(context.Background(), nil)
		assert.Nil(t, err)
	}
}
