// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"encoding/base64"
	"testing"

	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
)

func FuzzValidate(f *testing.F) {
	f.Add([]byte(``), "", "", "")
	f.Add([]byte(`not json`), "refs/heads/main", "def", "abc")
	f.Add([]byte(`{}`), "refs/heads/main", "def", "abc")
	f.Add([]byte(`{"subject":[]}`), "refs/heads/main", "def", "abc")
	f.Add([]byte(`{"subject":[null]}`), "refs/heads/main", "def", "abc")
	f.Add([]byte(`{"subject":[{"digest":{"gitTree":"abc"}}]}`), "refs/heads/main", "def", "abc")
	f.Add([]byte(`{"subject":[{"digest":{"gitTree":"abc"}}],"predicate":null}`), "refs/heads/main", "def", "abc")
	f.Add([]byte(`{"subject":[{"digest":{"gitTree":"abc"}}],"predicate":{"targetID":"abc","fromID":"def","targetRef":"refs/heads/main"}}`), "refs/heads/main", "def", "abc")
	f.Add([]byte(`{"subject":[{"digest":{"gitCommit":"abc"}}],"predicate":{"targetID":"abc","fromID":"def","targetRef":"refs/tags/v1"}}`), "refs/tags/v1", "def", "abc")

	f.Fuzz(func(_ *testing.T, payload []byte, targetRef, fromID, targetID string) {
		env := &sslibdsse.Envelope{
			Payload: base64.StdEncoding.EncodeToString(payload),
		}
		_ = Validate(env, targetRef, fromID, targetID)
	})
}
