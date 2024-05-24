// SPDX-License-Identifier: Apache-2.0

package dsse

import (
	"context"
	"encoding/base64"
	"path"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

func TestCreateEnvelope(t *testing.T) {
	rootMetadata := tuf.NewRootMetadata()
	env, err := CreateEnvelope(rootMetadata)
	assert.Nil(t, err)
	assert.Equal(t, PayloadType, env.PayloadType)
	assert.Equal(t, "eyJ0eXBlIjoicm9vdCIsInNwZWNfdmVyc2lvbiI6IjEuMCIsImNvbnNpc3RlbnRfc25hcHNob3QiOnRydWUsInZlcnNpb24iOjAsImV4cGlyZXMiOiIiLCJrZXlzIjpudWxsLCJyb2xlcyI6bnVsbH0=", env.Payload)
}

func TestSignEnvelope(t *testing.T) {
	signer, err := loadSigner("rsa")
	if err != nil {
		t.Fatal(err)
	}

	env, err := createSignedEnvelope(signer)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, env.Signatures, 1)
	assert.Equal(t, "SHA256:ESJezAOo+BsiEpddzRXS6+wtF16FID4NCd+3gj96rFo", env.Signatures[0].KeyID)

	env, err = SignEnvelope(context.Background(), env, signer)
	assert.Nil(t, err)
	assert.Len(t, env.Signatures, 1)
	assert.Equal(t, "SHA256:ESJezAOo+BsiEpddzRXS6+wtF16FID4NCd+3gj96rFo", env.Signatures[0].KeyID)
}

func TestVerifyEnvelope(t *testing.T) {
	signer, err := loadSigner("rsa")
	if err != nil {
		t.Fatal(err)
	}

	env, err := createSignedEnvelope(signer)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, VerifyEnvelope(context.Background(), env, []sslibdsse.Verifier{signer.Key}, 1))
}

func loadSigner(filename string) (*ssh.Signer, error) {
	dir, _ := filepath.Abs("../../testartifacts/testdata/keys/ssh")
	keyPath := path.Join(dir, filename)

	key, err := ssh.Import(keyPath)
	if err != nil {
		return nil, err
	}

	signer := &ssh.Signer{
		Key:  key,
		Path: keyPath,
	}

	return signer, nil
}

func createSignedEnvelope(signer *ssh.Signer) (*sslibdsse.Envelope, error) {

	message := []byte("test payload")
	payload := base64.StdEncoding.EncodeToString(message)

	env := &sslibdsse.Envelope{
		PayloadType: "application/vnd.gittuf+text",
		Payload:     payload,
		Signatures:  []sslibdsse.Signature{},
	}

	env, err := SignEnvelope(context.Background(), env, signer)
	if err != nil {
		return nil, err
	}

	return env, nil
}
