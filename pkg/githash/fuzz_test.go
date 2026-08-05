// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package githash

import "testing"

func FuzzNewHash(f *testing.F) {
	f.Add("")
	f.Add("not a hash")
	f.Add("0000000000000000000000000000000000000000")
	f.Add("abcdef12345678900987654321fedcbaabcdef12")
	f.Add("ABCDEF12345678900987654321FEDCBAABCDEF12")
	f.Add("0000000000000000000000000000000000000000000000000000000000000000")
	f.Add("abcdef12345678900987654321fedcbaabcdef1234567890abcdef1234567890")
	f.Add("zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz")

	f.Fuzz(func(_ *testing.T, h string) {
		_, _ = NewHash(h)
	})
}
