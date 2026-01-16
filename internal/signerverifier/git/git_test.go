// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSignerFromGitConfig(t *testing.T) {
	t.Run("gpg", func(t *testing.T) {
		// Make a test GPG keyring in tempdir to use for tests
		gpg.SetupTestGPGHomeDir(t, artifacts.GPGKey1Private, artifacts.GPGKey2Private)

		// Test GPG key fingerprints
		fingerprintGPG1 := "157507bbe151e378ce8126c1dcfe043cdd2db96e"
		fingerprintGPG2 := "7707e87f10df498472babc32e517e211cb23a9e9"

		t.Run("no signing method specified", func(t *testing.T) {
			// Test no configuration, this means GPG
			tmpDir := t.TempDir()
			repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

			if err := repo.SetGitConfig("gpg.format", ""); err != nil {
				t.Fatal(err)
			}
			if err := repo.SetGitConfig("user.signingkey", ""); err != nil {
				t.Fatal(err)
			}

			// No signingkey specified -> error
			_, err := LoadSignerFromGitConfig(repo)
			assert.ErrorIs(t, err, ErrSigningKeyNotSpecified)
		})

		t.Run("method specified but no signing key specified", func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

			if err := repo.SetGitConfig("gpg.format", "gpg"); err != nil {
				t.Fatal(err)
			}
			if err := repo.SetGitConfig("user.signingkey", ""); err != nil {
				t.Fatal(err)
			}

			_, err := LoadSignerFromGitConfig(repo)
			assert.ErrorIs(t, err, ErrSigningKeyNotSpecified)
		})

		t.Run("no method specified but signing key specified", func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

			if err := repo.SetGitConfig("gpg.format", ""); err != nil {
				t.Fatal(err)
			}
			if err := repo.SetGitConfig("user.signingkey", fingerprintGPG1); err != nil {
				t.Fatal(err)
			}

			signer, err := LoadSignerFromGitConfig(repo)
			assert.Nil(t, err)

			keyID, err := signer.KeyID()
			require.Nil(t, err)
			assert.Equal(t, fingerprintGPG1, keyID)
		})

		t.Run("method and signing key specified", func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

			if err := repo.SetGitConfig("gpg.format", "gpg"); err != nil {
				t.Fatal(err)
			}
			if err := repo.SetGitConfig("user.signingkey", fingerprintGPG2); err != nil {
				t.Fatal(err)
			}

			signer, err := LoadSignerFromGitConfig(repo)
			assert.Nil(t, err)

			keyID, err := signer.KeyID()
			require.Nil(t, err)
			assert.Equal(t, fingerprintGPG2, keyID)
		})
	})

	t.Run("ssh", func(t *testing.T) {
		t.Run("ssh key configured, but no signing key specified", func(t *testing.T) {
			// Test misconfiguration of SSH
			tmpDir := t.TempDir()
			// CreateTestGitRepository sets up the repository to use ssh by default
			repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

			if err := repo.SetGitConfig("user.signingkey", ""); err != nil {
				t.Fatal(err)
			}

			_, err := LoadSignerFromGitConfig(repo)
			assert.ErrorIs(t, err, ErrSigningKeyNotSpecified)
		})

		t.Run("ssh key specified", func(t *testing.T) {
			// Test a working SSH key configured
			tmpDir := t.TempDir()
			// CreateTestGitRepository sets up the repository to use ssh by default
			repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

			signer, err := LoadSignerFromGitConfig(repo)
			assert.Nil(t, err)

			compareKey := artifacts.SSHRSAPrivate

			compareSigner := ssh.NewKeyFromBytes(t, compareKey)
			require.Nil(t, err)
			signerKeyID, err := signer.KeyID()
			require.Nil(t, err)
			assert.Equal(t, compareSigner.KeyID, signerKeyID)
		})
	})

	// We can't test sigstore due to it being online...
}
