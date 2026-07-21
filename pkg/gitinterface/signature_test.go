// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"os"
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanSign(t *testing.T) {
	// Isolate from the developer's global and system git config so a host
	// with user.signingkey or gpg.format set in ~/.gitconfig does not leak
	// into the temp repos through scoped config lookups. os.DevNull resolves
	// to the platform-appropriate null device (NUL on Windows).
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_SYSTEM", os.DevNull)

	tests := map[string]struct {
		config        map[string]string
		expectedError error
	}{
		"explicit gpg, no key": {
			config: map[string]string{
				"gpg.format": "gpg",
			},
		},
		"explicit gpg, explicit key": {
			config: map[string]string{
				"gpg.format":      "gpg",
				"user.signingkey": "gpg-fingerprint",
			},
		},
		"no signing method, explicit key": {
			config: map[string]string{
				"user.signingkey": "gpg-fingerprint",
			},
		},
		"explicit ssh, explicit key": {
			config: map[string]string{
				"gpg.format":      "ssh",
				"user.signingkey": "ssh/key/path",
			},
		},
		"explicit ssh, no key": {
			config: map[string]string{
				"gpg.format": "ssh",
			},
			expectedError: ErrSigningKeyNotSpecified,
		},
		"explicit x509, no key": {
			config: map[string]string{
				"gpg.format": "x509",
			},
		},
		"explicit x509, explicit key": {
			config: map[string]string{
				"gpg.format":      "x509",
				"user.signingkey": "x509-signing-info",
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := setupRepository(t, tmpDir, false, ObjectFormatSHA1) // explicitly not using CreateTestGitRepository as that includes signing configurations

			for key, value := range test.config {
				if err := repo.SetGitConfig(key, value); err != nil {
					t.Fatal(err)
				}
			}

			err := repo.CanSign()
			assert.ErrorIs(t, err, test.expectedError, fmt.Sprintf("unexpected result in test '%s'", name))
		})
	}
}

func TestGetObjectSignature(t *testing.T) {
	t.Parallel()
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	treeBuilder := NewTreeBuilder(repo)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
	require.Nil(t, err)

	t.Run("signed commit", func(t *testing.T) {
		t.Parallel()
		commitID, err := repo.CommitUsingSpecificKey(emptyTreeHash, "refs/heads/signed", "Signed commit\n", artifacts.SSHED25519Private)
		require.Nil(t, err)

		payload, signature, err := repo.GetObjectSignature(commitID)
		assert.Nil(t, err)
		assert.Contains(t, string(payload), "tree "+emptyTreeHash.String())
		assert.NotContains(t, string(payload), "gpgsig")
		assert.Contains(t, string(signature), "-----BEGIN SSH SIGNATURE-----")
	})

	t.Run("unsigned commit", func(t *testing.T) {
		t.Parallel()
		commitID, err := repo.Commit(emptyTreeHash, "refs/heads/unsigned", "Unsigned commit\n", false)
		require.Nil(t, err)

		payload, signature, err := repo.GetObjectSignature(commitID)
		assert.Nil(t, err)
		assert.NotEmpty(t, payload)
		assert.Empty(t, signature)
	})

	t.Run("signed tag", func(t *testing.T) {
		t.Parallel()
		commitID, err := repo.Commit(emptyTreeHash, "refs/heads/tagged", "Commit to tag\n", false)
		require.Nil(t, err)
		tagID, err := repo.TagUsingSpecificKey(commitID, "v1-signed", "Signed tag\n", artifacts.SSHED25519Private)
		require.Nil(t, err)

		payload, signature, err := repo.GetObjectSignature(tagID)
		assert.Nil(t, err)
		assert.Contains(t, string(payload), "tag v1-signed")
		assert.NotContains(t, string(payload), "SSH SIGNATURE")
		assert.Contains(t, string(signature), "-----BEGIN SSH SIGNATURE-----")
	})

	t.Run("not a commit or tag", func(t *testing.T) {
		t.Parallel()
		blobID, err := repo.WriteBlob([]byte("test"))
		require.Nil(t, err)

		_, _, err = repo.GetObjectSignature(blobID)
		assert.ErrorIs(t, err, ErrNotCommitOrTag)
	})

	t.Run("signed commit, SHA-256 repository", func(t *testing.T) {
		t.Parallel()
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false, WithObjectFormat(ObjectFormatSHA256))

		emptyTreeHash, err := NewTreeBuilder(repo).WriteTreeFromEntries(nil)
		require.Nil(t, err)

		commitID, err := repo.CommitUsingSpecificKey(emptyTreeHash, "refs/heads/signed", "Signed commit\n", artifacts.SSHED25519Private)
		require.Nil(t, err)

		payload, signature, err := repo.GetObjectSignature(commitID)
		assert.Nil(t, err)
		assert.Contains(t, string(payload), "tree "+emptyTreeHash.String())
		assert.NotContains(t, string(payload), "gpgsig")
		assert.Contains(t, string(signature), "-----BEGIN SSH SIGNATURE-----")
	})
}
