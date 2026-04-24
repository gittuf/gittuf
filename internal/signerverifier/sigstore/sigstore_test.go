// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sigstore

import (
	"testing"

	signeropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/signer"
	verifieropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/verifier"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestVerifier(t *testing.T) {
	t.Run("KeyID", func(t *testing.T) {
		v := &Verifier{
			identity: "user@example.com",
			issuer:   "issuer",
		}

		keyID, err := v.KeyID()
		require.NoError(t, err)
		assert.Equal(t, "user@example.com::issuer", keyID)
	})

	t.Run("ExpectedExtensionKind", func(t *testing.T) {
		v := &Verifier{}
		assert.Equal(t, ExtensionMimeType, v.ExpectedExtensionKind())
	})

	t.Run("SetExtension", func(t *testing.T) {
		v := &Verifier{}
		v.SetExtension(nil)

		assert.Nil(t, v.ext)
	})
}

func TestNewVerifierFromIdentityAndIssuer(t *testing.T) {
	t.Run("default options", func(t *testing.T) {
		v := NewVerifierFromIdentityAndIssuer("id", "issuer")

		assert.Equal(t, "id", v.identity)
		assert.Equal(t, "issuer", v.issuer)
	})

	t.Run("with options", func(t *testing.T) {
		v := NewVerifierFromIdentityAndIssuer(
			"id",
			"issuer",
			verifieropts.WithRekorURL("rekor-url"),
		)

		assert.Equal(t, "rekor-url", v.rekorURL)
	})

	t.Run("empty identity issuer", func(t *testing.T) {
		v := NewVerifierFromIdentityAndIssuer("", "")
		assert.Equal(t, "", v.identity)
		assert.Equal(t, "", v.issuer)
	})
}

func TestSigner(t *testing.T) {
	t.Run("NewSigner options propagation", func(t *testing.T) {
		s := NewSigner(
			signeropts.WithIssuerURL("issuer-url"),
			signeropts.WithClientID("client-id"),
			signeropts.WithRedirectURL("redirect-url"),
			signeropts.WithFulcioURL("fulcio-url"),
			signeropts.WithRekorURL("rekor-url"),
		)

		assert.Equal(t, "issuer-url", s.issuerURL)
		assert.Equal(t, "client-id", s.clientID)
		assert.Equal(t, "redirect-url", s.redirectURL)
		assert.Equal(t, "fulcio-url", s.fulcioURL)
		assert.Equal(t, "rekor-url", s.rekorURL)

		require.NotNil(t, s.Verifier)
		assert.Equal(t, "rekor-url", s.Verifier.rekorURL)
	})

	t.Run("KeyID without token fetch", func(t *testing.T) {
		s := &Signer{
			Verifier: &Verifier{
				identity: "user",
				issuer:   "issuer",
			},
		}

		keyID, err := s.KeyID()
		require.NoError(t, err)
		assert.Equal(t, "user::issuer", keyID)
	})

	t.Run("MetadataKey", func(t *testing.T) {
		s := &Signer{
			Verifier: &Verifier{
				identity: "user",
				issuer:   "issuer",
			},
		}

		key, err := s.MetadataKey()
		require.NoError(t, err)

		assert.Equal(t, "user::issuer", key.KeyID)
		assert.Equal(t, KeyType, key.KeyType)
		assert.Equal(t, KeyScheme, key.Scheme)
		assert.Equal(t, "user", key.KeyVal.Identity)
		assert.Equal(t, "issuer", key.KeyVal.Issuer)
	})
}
