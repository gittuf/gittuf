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
		env, err := CreateEnvelope(func() {})

		assert.Nil(t, env)
		assert.ErrorContains(t, err, "json: unsupported type: func()")
	})
}

func TestSignEnvelope(t *testing.T) {
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

	assert.Len(t, env.Signatures, 1)
	assert.Equal(t, "SHA256:oNYBImx035m3rl1Sn/+j5DPrlS9+zXn7k3mjNrC5eto", env.Signatures[0].KeyID)

	env, err = SignEnvelope(context.Background(), env, signer)
	assert.Nil(t, err)
	assert.Len(t, env.Signatures, 1)
	assert.Equal(t, "SHA256:oNYBImx035m3rl1Sn/+j5DPrlS9+zXn7k3mjNrC5eto", env.Signatures[0].KeyID)

	t.Run("key ID error", func(t *testing.T) {
		expectedErr := errors.New("key ID error")
		env := newTestEnvelope()

		signedEnv, err := SignEnvelope(t.Context(), env, &keyIDErrorSigner{err: expectedErr})

		assert.Nil(t, signedEnv)
		assert.ErrorIs(t, err, expectedErr)
	})

	t.Run("invalid payload encoding", func(t *testing.T) {
		env := newTestEnvelope()
		env.Payload = "not base64%%%"

		signedEnv, err := SignEnvelope(t.Context(), env, signer)

		assert.Nil(t, signedEnv)
		assert.ErrorContains(t, err, "illegal base64 data")
	})

	t.Run("signing error", func(t *testing.T) {
		env := newTestEnvelope()
		invalidSigner := *signer
		invalidSigner.Path = filepath.Join(t.TempDir(), "missing-key")

		signedEnv, err := SignEnvelope(t.Context(), env, &invalidSigner)

		assert.Nil(t, signedEnv)
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("replace signatures from same key", func(t *testing.T) {
		env := newTestEnvelope()
		otherSignature := base64.StdEncoding.EncodeToString([]byte("other-signature"))
		env.Signatures = []sslibdsse.Signature{
			{KeyID: "other-key", Sig: otherSignature},
			{KeyID: keyID, Sig: "old-signature"},
			{KeyID: keyID, Sig: "duplicate-signature"},
		}

		signedEnv, err := SignEnvelope(t.Context(), env, signer)

		assert.NoError(t, err)
		assert.Len(t, signedEnv.Signatures, 2)
		assert.Equal(t, sslibdsse.Signature{KeyID: "other-key", Sig: otherSignature}, signedEnv.Signatures[0])
		assert.Equal(t, keyID, signedEnv.Signatures[1].KeyID)
		assert.NotEqual(t, "old-signature", signedEnv.Signatures[1].Sig)
		assert.NotEqual(t, "duplicate-signature", signedEnv.Signatures[1].Sig)

		acceptedKeys, err := VerifyEnvelope(t.Context(), signedEnv, []sslibdsse.Verifier{signer.Verifier}, 1)
		assert.NoError(t, err)
		if assert.Len(t, acceptedKeys, 1) {
			assert.Equal(t, keyID, acceptedKeys[0].KeyID)
		}
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

type keyIDErrorSigner struct {
	err error
}

func (s *keyIDErrorSigner) KeyID() (string, error) {
	return "", s.err
}

func (s *keyIDErrorSigner) Sign(context.Context, []byte) ([]byte, error) {
	return nil, nil
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
