// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package ssh

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/elliptic"
	"crypto/rand"
	"errors"
	"io"
	"os"
	"path/filepath"
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/hiddeco/sshsig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/crypto/ssh"
)

func TestNewSignerFromCryptoSigner(t *testing.T) {
	t.Parallel()

	_, ed25519Priv, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	ecdsaPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tests := map[string]crypto.Signer{
		"ed25519": ed25519Priv,
		"ecdsa":   ecdsaPriv,
	}

	for name, key := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			signer, err := NewSignerFromCryptoSigner(key)
			require.NoError(t, err)

			message := []byte("test message")
			signature, err := signer.Sign(context.Background(), message)
			require.NoError(t, err)

			require.NoError(t, signer.Verify(context.Background(), message, signature))
			assert.Error(t, signer.Verify(context.Background(), []byte("other message"), signature))

			sshPub, err := ssh.NewPublicKey(key.Public())
			require.NoError(t, err)

			keyID, err := signer.KeyID()
			require.NoError(t, err)
			assert.Equal(t, ssh.FingerprintSHA256(sshPub), keyID)

			verifier, err := NewVerifierFromKey(signer.MetadataKey())
			require.NoError(t, err)
			assert.NoError(t, verifier.Verify(context.Background(), message, signature))
		})
	}
}

func TestNewSignerFromCryptoSignerNilKey(t *testing.T) {
	t.Parallel()

	_, err := NewSignerFromCryptoSigner(nil)
	assert.Error(t, err)
}

func cryptoSignerFromPEM(t *testing.T, keyBytes []byte) crypto.Signer {
	t.Helper()

	raw, err := ssh.ParseRawPrivateKey(keyBytes)
	require.NoError(t, err)

	if key, ok := raw.(*ed25519.PrivateKey); ok {
		return *key
	}

	key, ok := raw.(crypto.Signer)
	require.True(t, ok, "parsed key %T is not a crypto.Signer", raw)

	return key
}

func TestInMemorySignerMatchesFileSigner(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		private []byte
		public  []byte
		// ECDSA draws a random nonce per signature, so only RSA and ed25519
		// yield the same bytes twice for the same key and message.
		deterministic bool
	}{
		"rsa":     {private: artifacts.SSHRSAPrivate, public: artifacts.SSHRSAPublicSSH, deterministic: true},
		"ecdsa":   {private: artifacts.SSHECDSAPrivate, public: artifacts.SSHECDSAPublicSSH},
		"ed25519": {private: artifacts.SSHED25519Private, public: artifacts.SSHED25519PublicSSH, deterministic: true},
	}

	data := []byte("DATA")

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			// ssh-keygen -Y sign reads the public key alongside the private one.
			keyPath := filepath.Join(t.TempDir(), name)
			require.NoError(t, os.WriteFile(keyPath, test.private, 0o600))
			require.NoError(t, os.WriteFile(keyPath+".pub", test.public, 0o600))

			fileSigner, err := NewSignerFromFile(keyPath)
			require.NoError(t, err)

			memorySigner, err := NewSignerFromCryptoSigner(cryptoSignerFromPEM(t, test.private))
			require.NoError(t, err)

			fileKeyID, err := fileSigner.KeyID()
			require.NoError(t, err)
			memoryKeyID, err := memorySigner.KeyID()
			require.NoError(t, err)
			assert.Equal(t, fileKeyID, memoryKeyID)
			assert.Equal(t, fileSigner.MetadataKey(), memorySigner.MetadataKey())
			assert.Equal(t, fileSigner.Public(), memorySigner.Public())

			fileSig, err := fileSigner.Sign(context.Background(), data)
			require.NoError(t, err)
			memorySig, err := memorySigner.Sign(context.Background(), data)
			require.NoError(t, err)

			assert.NoError(t, memorySigner.Verify(context.Background(), data, fileSig))
			assert.NoError(t, fileSigner.Verify(context.Background(), data, memorySig))

			parsedFileSig, err := sshsig.Unarmor(fileSig)
			require.NoError(t, err)
			parsedMemorySig, err := sshsig.Unarmor(memorySig)
			require.NoError(t, err)

			assert.Equal(t, parsedFileSig.Namespace, parsedMemorySig.Namespace)
			assert.Equal(t, parsedFileSig.HashAlgorithm, parsedMemorySig.HashAlgorithm)
			assert.Equal(t, parsedFileSig.Signature.Format, parsedMemorySig.Signature.Format)
			if test.deterministic {
				assert.Equal(t, parsedFileSig.Marshal(), parsedMemorySig.Marshal())
			}
		})
	}
}

// unsupportedSigner is a crypto.Signer holding a key type SSH cannot represent.
type unsupportedSigner struct{}

func (unsupportedSigner) Public() crypto.PublicKey { return struct{}{} }

func (unsupportedSigner) Sign(io.Reader, []byte, crypto.SignerOpts) ([]byte, error) {
	return nil, errors.New("unsupported")
}

func TestNewSignerFromCryptoSignerUnsupportedKey(t *testing.T) {
	t.Parallel()

	_, err := NewSignerFromCryptoSigner(unsupportedSigner{})
	assert.Error(t, err)
}

func TestNewKeyFromPublicKey(t *testing.T) {
	t.Parallel()

	pub, _, err := ed25519.GenerateKey(rand.Reader)
	require.NoError(t, err)

	sshPub, err := ssh.NewPublicKey(pub)
	require.NoError(t, err)

	key := NewKeyFromPublicKey(sshPub)
	assert.Equal(t, ssh.FingerprintSHA256(sshPub), key.KeyID)
	assert.Equal(t, KeyType, key.KeyType)
	assert.Equal(t, sshPub.Type(), key.Scheme)

	verifier, err := NewVerifierFromKey(key)
	require.NoError(t, err)

	verifierKeyID, err := verifier.KeyID()
	require.NoError(t, err)
	assert.Equal(t, key.KeyID, verifierKeyID)
}
