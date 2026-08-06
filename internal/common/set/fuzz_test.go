// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package set

import "testing"

func FuzzSetUnmarshalJSON(f *testing.F) {
	f.Add([]byte(`[]`))
	f.Add([]byte(`["a","b","a"]`))
	f.Add([]byte(`null`))
	f.Add([]byte(``))
	f.Add([]byte(`not json`))
	f.Add([]byte(`{"a":1}`))
	f.Add([]byte(`[1,2,3]`))

	f.Fuzz(func(_ *testing.T, data []byte) {
		s := NewSet[string]()
		_ = s.UnmarshalJSON(data)
	})
}
