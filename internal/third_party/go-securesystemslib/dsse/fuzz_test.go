package dsse

import "testing"

func FuzzB64Decode(f *testing.F) {
	f.Add("")
	f.Add("aGVsbG8=")      // "hello", standard encoding
	f.Add("aGVsbG8")       // missing padding
	f.Add("_-_-")          // URL-safe alphabet
	f.Add("+/+/")          // standard alphabet
	f.Add("!!!invalid!!!") // not base64 at all

	f.Fuzz(func(_ *testing.T, s string) {
		_, _ = b64Decode(s)
	})
}

func FuzzDecodeB64Payload(f *testing.F) {
	f.Add("")
	f.Add("aGVsbG8=")
	f.Add("eyJhIjoxfQ==") // {"a":1}

	f.Fuzz(func(_ *testing.T, payload string) {
		env := &Envelope{Payload: payload}
		_, _ = env.DecodeB64Payload()
	})
}
