// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCertsFromPath(t *testing.T) {
	t.Run("file not found", func(t *testing.T) {
		_, err := LoadCertsFromPath(filepath.Join(t.TempDir(), "missing.pem"))
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("invalid pem", func(t *testing.T) {
		pemPath := filepath.Join(t.TempDir(), "invalid.pem")
		err := os.WriteFile(pemPath, []byte("not a cert"), 0o600)
		require.Nil(t, err)

		_, err = LoadCertsFromPath(pemPath)
		assert.ErrorContains(t, err, "PEM decoding")
	})

	t.Run("success", func(t *testing.T) {
		pemPath := filepath.Join(t.TempDir(), "cert.pem")
		err := os.WriteFile(pemPath, createTestCertificatePEM(t), 0o600)
		require.Nil(t, err)

		certs, err := LoadCertsFromPath(pemPath)
		assert.Nil(t, err)
		assert.Len(t, certs, 1)
	})
}

func createTestCertificatePEM(t *testing.T) []byte {
	t.Helper()

	privateKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	require.Nil(t, err)

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
	require.Nil(t, err)

	return pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: derBytes})
}
