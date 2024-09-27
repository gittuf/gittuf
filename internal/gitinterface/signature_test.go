// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
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
}
