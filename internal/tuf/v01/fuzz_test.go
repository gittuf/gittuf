// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"encoding/json"
	"testing"
)

func FuzzRootMetadataUnmarshalJSON(f *testing.F) {
	if data, err := json.Marshal(NewRootMetadata()); err == nil {
		f.Add(data)
	}

	f.Add([]byte(``))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"type":"root","globalRules":[{}]}`))
	f.Add([]byte(`{"type":"root","propagations":[{}]}`))
	f.Add([]byte(`{"keys":{"a":{"keyid":"a"}},"roles":{}}`))

	f.Fuzz(func(_ *testing.T, data []byte) {
		r := &RootMetadata{}
		_ = r.UnmarshalJSON(data)
	})
}
