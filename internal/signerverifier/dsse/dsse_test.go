// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dsse

import (
	"context"
	"encoding/base64"
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/common"
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
	assert.Equal(t, "eyJ0eXBlIjoicm9vdCIsImV4cGlyZXMiOiIiLCJ2ZXJzaW9uIjoxLCJrZXlzIjpudWxsLCJyb2xlcyI6bnVsbH0=", env.Payload)

	t.Run("marshal error", func(t *testing.T) {
		expectedErr := errors.New("marshal error")

		env, err := CreateEnvelope(&marshalErrorValue{err: expectedErr})

		assert.Nil(t, env)
		assert.ErrorIs(t, err, expectedErr)
	})
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

	t.Run("key ID error", func(t *testing.T) {
		expectedErr := errors.New("key ID error")
		env := newTestEnvelope()

		signedEnv, err := SignEnvelope(t.Context(), env, &testSigner{keyIDErr: expectedErr})

		assert.Nil(t, signedEnv)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("invalid payload encoding", func(t *testing.T) {
		env := newTestEnvelope()
		env.Payload = "not base64%%%"

		signedEnv, err := SignEnvelope(t.Context(), env, &testSigner{keyID: "key"})

		assert.Nil(t, signedEnv)
		assert.ErrorContains(t, err, "illegal base64 data")
	})

	t.Run("signing error", func(t *testing.T) {
		expectedErr := errors.New("signing error")
		env := newTestEnvelope()

		signedEnv, err := SignEnvelope(t.Context(), env, &testSigner{keyID: "key", signErr: expectedErr})

		assert.Nil(t, signedEnv)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("replace signatures from same key", func(t *testing.T) {
		env := newTestEnvelope()
		env.Signatures = []sslibdsse.Signature{
			{KeyID: "other-key", Sig: "other-signature"},
			{KeyID: "key", Sig: "old-signature"},
			{KeyID: "key", Sig: "duplicate-signature"},
		}

		signedEnv, err := SignEnvelope(t.Context(), env, &testSigner{keyID: "key", signature: []byte("new-signature")})

		assert.NoError(t, err)
		assert.Equal(t, []sslibdsse.Signature{
			{KeyID: "other-key", Sig: "other-signature"},
			{KeyID: "key", Sig: base64.StdEncoding.EncodeToString([]byte("new-signature"))},
		}, signedEnv.Signatures)
	})
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

	acceptedKeys, err := VerifyEnvelope(t.Context(), env, []sslibdsse.Verifier{signer.Verifier}, 1)
	assert.Nil(t, err)
	assert.Equal(t, keyID, acceptedKeys[0].KeyID)

	t.Run("invalid threshold", func(t *testing.T) {
		env := &sslibdsse.Envelope{}

		acceptedKeys, err := VerifyEnvelope(t.Context(), env, nil, 0)

		assert.Nil(t, acceptedKeys)
		assert.ErrorIs(t, err, common.ErrInvalidThreshold)
	})

	t.Run("threshold not met", func(t *testing.T) {
		acceptedKeys, err := VerifyEnvelope(t.Context(), env, []sslibdsse.Verifier{signer.Verifier}, 2)

		assert.Len(t, acceptedKeys, 1)
		assert.ErrorContains(t, err, "accepted signatures do not match threshold")
	})

	t.Run("no verifiers", func(t *testing.T) {
		acceptedKeys, err := VerifyEnvelope(t.Context(), env, nil, 1)

		assert.Nil(t, acceptedKeys)
		assert.ErrorContains(t, err, "invalid threshold")
	})

	t.Run("nil envelope", func(t *testing.T) {
		acceptedKeys, err := VerifyEnvelope(t.Context(), nil, []sslibdsse.Verifier{signer.Verifier}, 1)

		assert.Nil(t, acceptedKeys)
		assert.ErrorContains(t, err, "cannot verify a nil envelope")
	})

	t.Run("no signatures", func(t *testing.T) {
		acceptedKeys, err := VerifyEnvelope(t.Context(), newTestEnvelope(), []sslibdsse.Verifier{signer.Verifier}, 1)

		assert.Nil(t, acceptedKeys)
		assert.ErrorIs(t, err, sslibdsse.ErrNoSignature)
	})

	t.Run("invalid payload encoding", func(t *testing.T) {
		invalidEnv := *env
		invalidEnv.Payload = "not base64%%%"

		acceptedKeys, err := VerifyEnvelope(t.Context(), &invalidEnv, []sslibdsse.Verifier{signer.Verifier}, 1)

		assert.Nil(t, acceptedKeys)
		assert.ErrorContains(t, err, "unable to base64 decode payload")
	})

	t.Run("invalid signature encoding", func(t *testing.T) {
		invalidEnv := *env
		invalidEnv.Signatures = []sslibdsse.Signature{{KeyID: keyID, Sig: "not base64%%%"}}

		acceptedKeys, err := VerifyEnvelope(t.Context(), &invalidEnv, []sslibdsse.Verifier{signer.Verifier}, 1)

		assert.Nil(t, acceptedKeys)
		assert.ErrorContains(t, err, "unable to base64 decode payload")
	})
}

type marshalErrorValue struct {
	err error
}

func (v *marshalErrorValue) MarshalJSON() ([]byte, error) {
	return nil, v.err
}

type testSigner struct {
	keyID     string
	keyIDErr  error
	signature []byte
	signErr   error
}

func (s *testSigner) KeyID() (string, error) {
	return s.keyID, s.keyIDErr
}

func (s *testSigner) Sign(context.Context, []byte) ([]byte, error) {
	return s.signature, s.signErr
}

func newTestEnvelope() *sslibdsse.Envelope {
	return &sslibdsse.Envelope{
		PayloadType: PayloadType,
		Payload:     base64.StdEncoding.EncodeToString([]byte("test payload")),
	}
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
