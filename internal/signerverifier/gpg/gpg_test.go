// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gpg

import (
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestGPG(t *testing.T) {
	// Make a test GPG keyring in tempdir to use for tests
	SetupTestGPGHomeDir(t, artifacts.GPGKey1Private, artifacts.GPGKey2Private)

	// Test GPG key fingerprints
	fingerprintGPG1 := "157507bbe151e378ce8126c1dcfe043cdd2db96e"
	fingerprintGPG2 := "7707e87f10df498472babc32e517e211cb23a9e9"

	tests := []struct {
		pubkeyBytes []byte
		keyID       string
	}{
		{
			pubkeyBytes: artifacts.GPGKey1Public,
			keyID:       fingerprintGPG1,
		},
		{
			pubkeyBytes: artifacts.GPGKey2Public,
			keyID:       fingerprintGPG2,
		},
	}

	data := []byte("DATA")
	notData := []byte("NOT DATA")

	// Run tests
	for _, test := range tests {
		t.Run(test.keyID, func(t *testing.T) {
			// load publ key for verifier
			pubKey, err := LoadGPGKeyFromBytes(test.pubkeyBytes)
			if err != nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}
			assert.Equal(t, test.keyID, pubKey.KeyID)

			verifier, err := NewVerifierFromKey(pubKey)
			if err != nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}

			signer, err := NewSignerFromKeyID(test.keyID)
			if err != nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}

			sig, err := signer.Sign(t.Context(), data)
			if err != nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}

			err = verifier.Verify(t.Context(), data, sig)
			if err != nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}

			err = verifier.Verify(t.Context(), notData, sig)
			if err == nil {
				t.Fatalf("%s: %v", test.keyID, err)
			}
		})
	}
}

func TestLoadGPGKeyFromBytes(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		key, err := LoadGPGKeyFromBytes(artifacts.GPGKey1Public)
		assert.Nil(t, err)
		assert.Equal(t, KeyType, key.KeyType)
		assert.Equal(t, KeyType, key.Scheme)
		assert.Equal(t, "157507bbe151e378ce8126c1dcfe043cdd2db96e", key.KeyID)
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := LoadGPGKeyFromBytes([]byte("not a gpg key"))
		assert.ErrorContains(t, err, "no armored data found")
	})
}

func TestMSYSPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want string
	}{
		{
			name: "Windows path with backslashes",
			path: `C:\Users\foo\AppData\Local\Temp\gittuf-gpg-123`,
			want: "/c/Users/foo/AppData/Local/Temp/gittuf-gpg-123",
		},
		{
			name: "Windows path with forward slashes",
			path: "C:/Users/foo/AppData/Local/Temp/gittuf-gpg-123",
			want: "/c/Users/foo/AppData/Local/Temp/gittuf-gpg-123",
		},
		{
			name: "uppercase drive letter is lowercased",
			path: `D:\gittuf-gpg-123`,
			want: "/d/gittuf-gpg-123",
		},
		{
			name: "already POSIX-style path is unchanged",
			path: "/tmp/gittuf-gpg-123",
			want: "/tmp/gittuf-gpg-123",
		},
		{
			name: "relative path without a drive letter is unchanged",
			path: "gittuf-gpg-123",
			want: "gittuf-gpg-123",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, msysPath(test.path))
		})
	}
}

func TestIsMSYSGPGPath(t *testing.T) {
	tests := []struct {
		name string
		path string
		want bool
	}{
		{
			name: "Git for Windows usr/bin build",
			path: `C:\Program Files\Git\usr\bin\gpg.exe`,
			want: true,
		},
		{
			name: "Git for Windows mingw64 build",
			path: `C:\Program Files\Git\mingw64\bin\gpg.exe`,
			want: true,
		},
		{
			name: "native Gpg4win install",
			path: `C:\Program Files\GnuPG\bin\gpg.exe`,
			want: false,
		},
		{
			name: "Linux system gpg is not mistaken for an MSYS build",
			path: "/usr/bin/gpg",
			want: false,
		},
		{
			name: "macOS Homebrew gpg",
			path: "/opt/homebrew/bin/gpg",
			want: false,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			assert.Equal(t, test.want, isMSYSGPGPath(test.path))
		})
	}
}

func TestNewVerifierFromKey(t *testing.T) {
	t.Run("valid", func(t *testing.T) {
		pubKey, err := LoadGPGKeyFromBytes(artifacts.GPGKey1Public)
		require.Nil(t, err)

		verifier, err := NewVerifierFromKey(pubKey)
		assert.Nil(t, err)
		require.NotNil(t, verifier)

		keyID, err := verifier.KeyID()
		assert.Nil(t, err)
		assert.Equal(t, pubKey.KeyID, keyID)

		assert.Equal(t, verifier.entity.PrimaryKey.PublicKey, verifier.Public())
		assert.Equal(t, "DCFE043CDD2DB96E", verifier.entity.PrimaryKey.KeyIdString())
		assert.Equal(t, pubKey, verifier.MetadataKey())
	})

	t.Run("invalid", func(t *testing.T) {
		_, err := NewVerifierFromKey(&signerverifier.SSLibKey{
			KeyID:   "bad",
			KeyType: KeyType,
			Scheme:  KeyType,
			KeyVal: signerverifier.KeyVal{
				Public: "invalid",
			},
		})
		assert.ErrorContains(t, err, "failed to parse gpg key")
	})
}
