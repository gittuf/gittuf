// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gpg

import "testing"

func FuzzLoadGPGKeyFromBytes(f *testing.F) {
	f.Add([]byte(""))
	f.Add([]byte("not a key"))
	f.Add([]byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\n\n-----END PGP PUBLIC KEY BLOCK-----\n"))
	f.Add([]byte("-----BEGIN PGP PUBLIC KEY BLOCK-----\ngarbage\n-----END PGP PUBLIC KEY BLOCK-----\n"))

	f.Fuzz(func(_ *testing.T, contents []byte) {
		_, _ = LoadGPGKeyFromBytes(contents)
	})
}
