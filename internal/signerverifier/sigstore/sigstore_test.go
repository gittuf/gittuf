// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sigstore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"math/big"
	"testing"
	"time"

	protobundle "github.com/sigstore/protobuf-specs/gen/pb-go/bundle/v1"
	protocommon "github.com/sigstore/protobuf-specs/gen/pb-go/common/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicKeyFromVerificationMaterialCertificate(t *testing.T) {
	certBytes, publicKey := createTestCertificate(t)

	material := &protobundle.VerificationMaterial{
		Content: &protobundle.VerificationMaterial_Certificate{
			Certificate: &protocommon.X509Certificate{RawBytes: certBytes},
		},
	}

	key, err := publicKeyFromVerificationMaterial(material)
	require.NoError(t, err)
	assert.Equal(t, publicKey, key)
}

func TestPublicKeyFromVerificationMaterialChain(t *testing.T) {
	certBytes, publicKey := createTestCertificate(t)

	material := &protobundle.VerificationMaterial{
		Content: &protobundle.VerificationMaterial_X509CertificateChain{
			X509CertificateChain: &protocommon.X509CertificateChain{
				Certificates: []*protocommon.X509Certificate{{RawBytes: certBytes}},
			},
		},
	}

	key, err := publicKeyFromVerificationMaterial(material)
	require.NoError(t, err)
	assert.Equal(t, publicKey, key)
}

func TestPublicKeyFromVerificationMaterialNil(t *testing.T) {
	key, err := publicKeyFromVerificationMaterial(nil)
	require.NoError(t, err)
	assert.Nil(t, key)
}

func TestPublicKeyFromVerificationMaterialEmptyChain(t *testing.T) {
	material := &protobundle.VerificationMaterial{
		Content: &protobundle.VerificationMaterial_X509CertificateChain{
			X509CertificateChain: &protocommon.X509CertificateChain{},
		},
	}

	key, err := publicKeyFromVerificationMaterial(material)
	require.NoError(t, err)
	assert.Nil(t, key)
}

func TestPublicKeyFromVerificationMaterialInvalidCertificate(t *testing.T) {
	material := &protobundle.VerificationMaterial{
		Content: &protobundle.VerificationMaterial_Certificate{
			Certificate: &protocommon.X509Certificate{RawBytes: []byte("not-a-cert")},
		},
	}

	_, err := publicKeyFromVerificationMaterial(material)
	assert.ErrorContains(t, err, "unable to parse Sigstore certificate")
}

func createTestCertificate(t *testing.T) ([]byte, any) {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.NoError(t, err)

	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName: "gittuf-test-cert",
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(1 * time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageKeyEncipherment,
		BasicConstraintsValid: true,
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, privateKey.Public(), privateKey)
	require.NoError(t, err)

	return derBytes, privateKey.Public()
}
