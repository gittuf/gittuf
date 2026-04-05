// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
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
		config map[string]string
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
			assert.Nil(t, err, fmt.Sprintf("unexpected result in test '%s'", name))
		})
	}

	t.Run("can sign with gpg format", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := setupRepository(t, tmpDir, false)

		err := repo.SetGitConfig("gpg.format", "gpg")
		require.NoError(t, err)

		err = repo.CanSign()
		assert.Nil(t, err)
	})

	t.Run("can sign with x509 format", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := setupRepository(t, tmpDir, false)

		err := repo.SetGitConfig("gpg.format", "x509")
		require.NoError(t, err)

		err = repo.CanSign()
		assert.Nil(t, err)
	})

	t.Run("can sign with openpgp format", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := setupRepository(t, tmpDir, false)

		err := repo.SetGitConfig("gpg.format", "openpgp")
		require.NoError(t, err)

		err = repo.CanSign()
		assert.Nil(t, err)
	})

	t.Run("ssh without key returns error", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := setupRepository(t, tmpDir, false)

		err := repo.SetGitConfig("gpg.format", "ssh")
		require.NoError(t, err)

		err = repo.CanSign()
		assert.NotNil(t, err)
		assert.Equal(t, ErrSigningKeyNotSpecified, err)
	})

	t.Run("ssh with key succeeds", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := setupRepository(t, tmpDir, false)

		err := repo.SetGitConfig("gpg.format", "ssh")
		require.NoError(t, err)
		err = repo.SetGitConfig("user.signingkey", "/path/to/key.pub")
		require.NoError(t, err)

		err = repo.CanSign()
		assert.Nil(t, err)
	})

	t.Run("default behavior without configuration", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := setupRepository(t, tmpDir, false)

		err := repo.CanSign()
		assert.Nil(t, err)
	})

	t.Run("various formats", func(t *testing.T) {
		formats := []string{"gpg", "ssh", "x509", "openpgp"}

		for _, format := range formats {
			t.Run(fmt.Sprintf("format_%s", format), func(t *testing.T) {
				tmpDir := t.TempDir()
				repo := setupRepository(t, tmpDir, false)

				err := repo.SetGitConfig("gpg.format", format)
				require.NoError(t, err)

				if format == "ssh" {
					err = repo.SetGitConfig("user.signingkey", "/path/to/key")
					require.NoError(t, err)
				}

				err = repo.CanSign()
				assert.Nil(t, err)
			})
		}
	})
}

func TestGetSigningMethod(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]string
		expected string
	}{
		{
			name:     "default to gpg when not set",
			config:   map[string]string{},
			expected: "gpg",
		},
		{
			name:     "explicit gpg",
			config:   map[string]string{"gpg.format": "gpg"},
			expected: "gpg",
		},
		{
			name:     "explicit ssh",
			config:   map[string]string{"gpg.format": "ssh"},
			expected: "ssh",
		},
		{
			name:     "explicit x509",
			config:   map[string]string{"gpg.format": "x509"},
			expected: "x509",
		},
		{
			name:     "explicit openpgp",
			config:   map[string]string{"gpg.format": "openpgp"},
			expected: "openpgp",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSigningMethod(tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}

func TestGetSigningKeyInfo(t *testing.T) {
	tests := []struct {
		name     string
		config   map[string]string
		expected string
	}{
		{
			name:     "no signing key set",
			config:   map[string]string{},
			expected: "",
		},
		{
			name:     "signing key set",
			config:   map[string]string{"user.signingkey": "my-key-id"},
			expected: "my-key-id",
		},
		{
			name:     "signing key with path",
			config:   map[string]string{"user.signingkey": "/path/to/key.pub"},
			expected: "/path/to/key.pub",
		},
		{
			name:     "signing key with fingerprint",
			config:   map[string]string{"user.signingkey": "ABCD1234EFGH5678"},
			expected: "ABCD1234EFGH5678",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := getSigningKeyInfo(tt.config)
			assert.Equal(t, tt.expected, result)
		})
	}
}
