// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"os"
	"path/filepath"
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCreateTestGitRepository(t *testing.T) {
	t.Run("configures test identity and signing key", func(t *testing.T) {
		tmpDir := t.TempDir()
		signingKeysDir := t.TempDir()

		repo, err := createTestGitRepository(tmpDir, signingKeysDir, false)
		require.Nil(t, err)

		config, err := repo.GetGitConfig()
		require.Nil(t, err)
		assert.Equal(t, testName, config["user.name"])
		assert.Equal(t, testEmail, config["user.email"])
		assert.Equal(t, filepath.Join(signingKeysDir, "key.pub"), config["user.signingkey"])
		assert.Equal(t, "ssh", config["gpg.format"])
	})

	t.Run("invalid object format", func(t *testing.T) {
		_, err := createTestGitRepository(t.TempDir(), t.TempDir(), false, WithObjectFormat("bogus"))
		assert.Error(t, err)
	})

	t.Run("invalid signing keys directory", func(t *testing.T) {
		signingKeysDir := filepath.Join(t.TempDir(), "keys")
		require.Nil(t, os.WriteFile(signingKeysDir, nil, 0o600))

		_, err := createTestGitRepository(t.TempDir(), signingKeysDir, false)
		assert.Error(t, err)
	})
}

func TestWriteSigningKeys(t *testing.T) {
	t.Run("writes rsa key pair", func(t *testing.T) {
		tmpDir := t.TempDir()

		require.Nil(t, writeSigningKeys(tmpDir))

		privateKey, err := os.ReadFile(filepath.Join(tmpDir, "key"))
		require.Nil(t, err)
		assert.Equal(t, artifacts.SSHRSAPrivate, privateKey)

		publicKey, err := os.ReadFile(filepath.Join(tmpDir, "key.pub"))
		require.Nil(t, err)
		assert.Equal(t, artifacts.SSHRSAPublicSSH, publicKey)
	})

	t.Run("private key write error", func(t *testing.T) {
		keysDir := filepath.Join(t.TempDir(), "keys")
		require.Nil(t, os.WriteFile(keysDir, nil, 0o600))

		assert.Error(t, writeSigningKeys(keysDir))
	})

	t.Run("public key write error", func(t *testing.T) {
		keysDir := t.TempDir()
		require.Nil(t, os.Mkdir(filepath.Join(keysDir, "key.pub"), 0o700))

		assert.Error(t, writeSigningKeys(keysDir))
	})
}
