// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

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
	f.Add([]byte(`{"type":"root","schemaVersion":"https://gittuf.dev/root/v0.2","principals":{"a":{"keyid":"a"}}}`))
	f.Add([]byte(`{"type":"root","globalRules":[{}],"propagations":[{}]}`))

	f.Fuzz(func(_ *testing.T, data []byte) {
		r := &RootMetadata{}
		_ = r.UnmarshalJSON(data)
	})
}

func FuzzDelegationsUnmarshalJSON(f *testing.F) {
	f.Add([]byte(``))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"principals":{},"roles":[]}`))
	f.Add([]byte(`{"principals":{"k":{"keyid":"abc"}},"roles":[]}`))
	f.Add([]byte(`{"principals":{"p":{"personID":"x"}},"roles":[]}`))
	f.Add([]byte(`{"principals":{"p":{}},"roles":[]}`))

	f.Fuzz(func(_ *testing.T, data []byte) {
		d := &Delegations{}
		_ = d.UnmarshalJSON(data)
	})
}

func FuzzOtherRepositoryUnmarshalJSON(f *testing.F) {
	f.Add([]byte(``))
	f.Add([]byte(`{}`))
	f.Add([]byte(`null`))
	f.Add([]byte(`{"name":"repo","location":"https://example.com","initialRootPrincipals":[]}`))
	f.Add([]byte(`{"name":"repo","initialRootPrincipals":[{"keyid":"abc"}]}`))
	f.Add([]byte(`{"initialRootPrincipals":[{"personID":"x"}]}`))

	f.Fuzz(func(_ *testing.T, data []byte) {
		o := &OtherRepository{}
		_ = o.UnmarshalJSON(data)
	})
}
