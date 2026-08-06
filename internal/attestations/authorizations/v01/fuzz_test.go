// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

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
	f.Add([]byte(`{"subject":[{"digest":{"gitTree":"abc"}}],"predicate":{"targetTreeID":"abc","fromRevisionID":"def","targetRef":"refs/heads/main"}}`), "refs/heads/main", "def", "abc")

	f.Fuzz(func(_ *testing.T, payload []byte, targetRef, fromRevisionID, targetTreeID string) {
		env := &sslibdsse.Envelope{
			Payload: base64.StdEncoding.EncodeToString(payload),
		}
		_ = Validate(env, targetRef, fromRevisionID, targetTreeID)
	})
}
