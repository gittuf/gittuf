// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dsse

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/assert"
)

func TestCreateEnvelope(t *testing.T) {
	rootMetadata := tufv01.NewRootMetadata()
	env, err := CreateEnvelope(rootMetadata)
	assert.Nil(t, err)
	assert.Equal(t, PayloadType, env.PayloadType)
	assert.Equal(t, "eyJ0eXBlIjoicm9vdCIsImV4cGlyZXMiOiIiLCJrZXlzIjpudWxsLCJyb2xlcyI6bnVsbH0=", env.Payload)
}

func TestSignEnvelope(t *testing.T) {
	keyPath := setupTestECDSAPair(t)

	signer, err := loadSSHSigner(keyPath)
	if err != nil {
		t.Fatal(err)
	}

	env, err := createSignedEnvelope(signer)
	if err != nil {
		t.Fatal(err)
	}

	assert.Len(t, env.Signatures, 1)
	assert.Equal(t, "SHA256:oNYBImx035m3rl1Sn/+j5DPrlS9+zXn7k3mjNrC5eto", env.Signatures[0].KeyID)

	env, err = SignEnvelope(context.Background(), env, signer)
	assert.Nil(t, err)
	assert.Len(t, env.Signatures, 1)
	assert.Equal(t, "SHA256:oNYBImx035m3rl1Sn/+j5DPrlS9+zXn7k3mjNrC5eto", env.Signatures[0].KeyID)
}

func TestVerifyEnvelope(t *testing.T) {
	keyPath := setupTestECDSAPair(t)

	signer, err := loadSSHSigner(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	keyID, err := signer.KeyID()
	if err != nil {
		t.Fatal(err)
	}

	env, err := createSignedEnvelope(signer)
	if err != nil {
		t.Fatal(err)
	}

	acceptedKeys, err := VerifyEnvelope(context.Background(), env, []sslibdsse.Verifier{signer.Verifier}, 1)
	assert.Nil(t, err)
	assert.Equal(t, keyID, acceptedKeys[0].KeyID)
}

func loadSSHSigner(keyPath string) (*ssh.Signer, error) {
	key, err := ssh.NewKeyFromFile(keyPath)
	if err != nil {
		return nil, err
	}
	verifier, err := ssh.NewVerifierFromKey(key)
	if err != nil {
		return nil, err
	}
	return &ssh.Signer{
		Verifier: verifier,
		Path:     keyPath,
	}, nil
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

func setupTestECDSAPair(t *testing.T) string {
	tmpDir := t.TempDir()
	privPath := filepath.Join(tmpDir, "ecdsa")

	if err := os.WriteFile(privPath, artifacts.SSHECDSAPrivate, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(privPath+".pub", artifacts.SSHECDSAPublicSSH, 0o600); err != nil {
		t.Fatal(err)
	}
	return privPath
}
