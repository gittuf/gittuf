// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCanSign(t *testing.T) {
	// Note: This is currently not testing the one scenario where CanSign
	// returns an error: when gpg.format=ssh but user.signingkey is undefined.
	// This is because on developer machines, there's a very good chance
	// user.signingkey is set globally, which gets picked up during the test.
	// This also means we can't reliably test the case when no signing specific
	// configuration is set (which defaults to gpg + the default key).
	// :(

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
