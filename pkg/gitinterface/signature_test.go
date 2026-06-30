// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"fmt"
	"os"
	"testing"

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
			repo := setupRepository(t, tmpDir, false) // explicitly not using CreateTestGitRepository as that includes signing configurations

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

func TestVerifySignature(t *testing.T) {
	t.Run("not a commit or a tag", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		blobID, err := repo.WriteBlob([]byte("test"))
		require.Nil(t, err)

		err = repo.VerifySignature(context.Background(), blobID, nil)
		assert.ErrorIs(t, err, ErrNotCommitOrTag)
	})
}
