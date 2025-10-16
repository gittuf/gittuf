// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSignerFromGitConfig(t *testing.T) {
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

		_, err := LoadSignerFromGitConfig(repo)
		assert.ErrorContains(t, err, "not specified")
	})

	t.Run("ssh key configured, but no signing key specified", func(t *testing.T) {
		// Test misconfiguration of SSH
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		if err := repo.SetGitConfig("user.signingkey", ""); err != nil {
			t.Fatal(err)
		}

		_, err := LoadSignerFromGitConfig(repo)
		assert.ErrorIs(t, err, ErrSigningKeyNotSpecified)
	})

	t.Run("gpg key specified", func(t *testing.T) {
		// Test GPG behavior, should error out due to not being implemented
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		if err := repo.SetGitConfig("gpg.format", "gpg"); err != nil {
			t.Fatal(err)
		}

		_, err := LoadSignerFromGitConfig(repo)
		assert.ErrorContains(t, err, "not implemented")
	})

	t.Run("ssh key specified", func(t *testing.T) {
		// Test a working SSH key configured
		tmpDir := t.TempDir()
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

	// We can't test sigstore due to it being online...
}
