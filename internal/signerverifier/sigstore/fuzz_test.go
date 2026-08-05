// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sigstore

import "testing"

func FuzzStringAsBoolUnmarshalJSON(f *testing.F) {
	f.Add([]byte(`true`))
	f.Add([]byte(`"true"`))
	f.Add([]byte(`True`))
	f.Add([]byte(`"True"`))
	f.Add([]byte(`false`))
	f.Add([]byte(`"false"`))
	f.Add([]byte(``))
	f.Add([]byte(`1`))
	f.Add([]byte(`"maybe"`))

	f.Fuzz(func(_ *testing.T, data []byte) {
		var b stringAsBool
		_ = b.UnmarshalJSON(data)
	})
}
