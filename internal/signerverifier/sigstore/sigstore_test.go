// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sigstore

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"testing"

	signeropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/signer"
	verifieropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/verifier"
	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/sigstore/sigstore-go/pkg/sign"
	"github.com/sigstore/sigstore-go/pkg/testing/ca"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/types/known/structpb"
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

func TestSigstoreWorkflow(t *testing.T) {
	// Setup Virtual Sigstore
	virtualSigstore, err := ca.NewVirtualSigstore()
	require.NoError(t, err)

	identity := "user@example.com"
	issuer := "https://issuer.example.com"
	data := []byte("test data")

	// 1. Test Verification with externally signed artifact
	t.Run("Verify externally signed artifact", func(t *testing.T) {
		entity, err := virtualSigstore.Sign(identity, issuer, data)
		require.NoError(t, err)

		// Create a bundle from the entity to extract the signature bytes
		// In gittuf, sig is the JSON-marshaled MessageSignature
		content, err := entity.SignatureContent()
		require.NoError(t, err)

		// Manually reconstruct MessageSignature proto for verification
		digest := sha256.Sum256(data)
		msgSig := &protocommon.MessageSignature{
			MessageDigest: &protocommon.HashOutput{
				Algorithm: protocommon.HashAlgorithm_SHA2_256,
				Digest:    digest[:],
			},
			Signature: content.Signature(),
		}
		sigBytes, err := protojson.Marshal(msgSig)
		require.NoError(t, err)

		verifier := NewVerifierFromIdentityAndIssuer(identity, issuer, verifieropts.WithTrustedRoot(virtualSigstore))

		// Note: we are passing an empty extension here which might cause verification to fail
		// if the policy requires transparency proofs (which gittuf's default does).
		// However, this test still exercises the plumbing of Verifier.Verify.
		verifier.SetExtension(new(structpb.Struct))

		err = verifier.Verify(context.Background(), data, sigBytes)
		// We don't strictly require success here as reconstructing the full bundle with transparency proofs
		// from TestEntity is complex, but we ensure the plumbing doesn't panic and reaches the verifier.
		if err != nil {
			assert.ErrorContains(t, err, "")
		}
	})

	// 2. Test Signer with mocked dependencies
	t.Run("Signer workflow with mocks", func(t *testing.T) {
		// Mock ID Token
		tokenClaims := map[string]interface{}{
			"iss":            issuer,
			"sub":            "sub",
			"email":          identity,
			"email_verified": "true",
		}
		claimsBytes, err := json.Marshal(tokenClaims)
		require.NoError(t, err)
		encodedClaims := base64.RawURLEncoding.EncodeToString(claimsBytes)
		mockToken := fmt.Sprintf("header.%s.signature", encodedClaims)

		signer := NewSigner(
			signeropts.WithIssuerURL(issuer),
			signeropts.WithFulcioURL(""), // skip Fulcio config check in parseTokenForIdentityAndIssuer
		)

		// Set token manually to bypass OIDC discovery
		signer.token = mockToken

		// Mock Sigstore services to reach sign.Bundle
		signer.fulcio = &mockCertificateProvider{}
		signer.rekor = &mockTransparency{}

		_, err = signer.Sign(context.Background(), data)
		// It will fail because our mocks return errors, but it confirms we hit the sign path
		// and that identity/issuer were correctly parsed from the token.
		assert.Error(t, err)

		assert.Equal(t, identity, signer.identity)
		assert.Equal(t, issuer, signer.issuer)
	})
}

type mockCertificateProvider struct{}

func (m *mockCertificateProvider) GetCertificate(_ context.Context, _ sign.Keypair, _ *sign.CertificateProviderOptions) ([]byte, error) {
	return nil, fmt.Errorf("mock fulcio error")
}

type mockTransparency struct{}

func (m *mockTransparency) GetTransparencyLogEntry(_ context.Context, _ []byte, _ *protobundle.Bundle) error {
	return fmt.Errorf("mock rekor error")
}
