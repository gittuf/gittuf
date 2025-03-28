// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package git

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadSignerFromGitConfig(t *testing.T) {
	// FIXME: implement tests for this...
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	// Test default test behavior: SSH key
	signer, err := LoadSignerFromGitConfig(repo)
	assert.Nil(t, err)

	compareKey := artifacts.SSHRSAPrivate

	privateKeyPath := filepath.Join(tmpDir, "key")
	if err := os.WriteFile(privateKeyPath, compareKey, 0o600); err != nil {
		t.Fatal(err)
	}
	compareSigner, err := ssh.NewSignerFromFile(privateKeyPath)
	require.Nil(t, err)

	compareKeyID, err := compareSigner.KeyID()
	require.Nil(t, err)
	signerKeyID, err := signer.KeyID()
	require.Nil(t, err)

	assert.Equal(t, compareKeyID, signerKeyID)

	// Test GPG behavior
	if err := repo.SetGitConfig("gpg.format", ""); err != nil {
		t.Fatal(err)
	}

	_, err = LoadSignerFromGitConfig(repo)
	assert.ErrorContains(t, err, "not implemented")

	// Test default behavior
	if err := repo.SetGitConfig("gpg.format", ""); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetGitConfig("user.signingkey", ""); err != nil {
		t.Fatal(err)
	}

	_, err = LoadSignerFromGitConfig(repo)
	assert.ErrorIs(t, err, ErrNoGitKeyConfigured)
}
