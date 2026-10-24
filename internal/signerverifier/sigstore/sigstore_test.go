package sigstore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func createTestKeys(t *testing.T) (string, string) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}

	pubBytes, err := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	if err != nil {
		t.Fatal(err)
	}
	pubPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: pubBytes,
	})

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization: []string{"Test"},
		},
		NotBefore:             time.Now(),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageCodeSigning},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatal(err)
	}
	certPEM := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: certBytes,
	})

	tmpDir := t.TempDir()
	certPath := tmpDir + "/cert.pem"
	pubPath := tmpDir + "/pub.pem"

	if err := os.WriteFile(certPath, certPEM, 0600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pubPath, pubPEM, 0600); err != nil {
		t.Fatal(err)
	}

	return certPath, pubPath
}

func TestGetTrustedMaterial(t *testing.T) {
	os.Unsetenv(EnvSigstoreRootFile)
	os.Unsetenv(EnvSigstoreCTLogPublicKeyFile)
	os.Unsetenv(EnvSigstoreRekorPublicKey)

	_, err := getTrustedMaterial("")
	assert.NoError(t, err)

	t.Run("invalid root file", func(t *testing.T) {
		os.Setenv(EnvSigstoreRootFile, "non_existent_file.pem")
		defer os.Unsetenv(EnvSigstoreRootFile)
		_, err = getTrustedMaterial("")
		assert.Error(t, err)
	})

	certPath, pubPath := createTestKeys(t)

	t.Run("valid root file, no others", func(t *testing.T) {
		os.Setenv(EnvSigstoreRootFile, certPath)
		defer os.Unsetenv(EnvSigstoreRootFile)
		_, err = getTrustedMaterial("")
		assert.NoError(t, err)
	})

	t.Run("invalid rekor key", func(t *testing.T) {
		os.Setenv(EnvSigstoreRekorPublicKey, "non_existent_file.pem")
		defer os.Unsetenv(EnvSigstoreRekorPublicKey)
		_, err = getTrustedMaterial("")
		assert.Error(t, err)
	})

	t.Run("valid rekor key, no others", func(t *testing.T) {
		os.Setenv(EnvSigstoreRekorPublicKey, pubPath)
		defer os.Unsetenv(EnvSigstoreRekorPublicKey)
		_, err = getTrustedMaterial("")
		assert.NoError(t, err)
	})

	t.Run("invalid ctlog key", func(t *testing.T) {
		os.Setenv(EnvSigstoreCTLogPublicKeyFile, "non_existent_file.pem")
		defer os.Unsetenv(EnvSigstoreCTLogPublicKeyFile)
		_, err = getTrustedMaterial("")
		assert.Error(t, err)
	})

	t.Run("valid ctlog key, no others", func(t *testing.T) {
		os.Setenv(EnvSigstoreCTLogPublicKeyFile, pubPath)
		defer os.Unsetenv(EnvSigstoreCTLogPublicKeyFile)
		_, err = getTrustedMaterial("")
		assert.NoError(t, err)
	})

	t.Run("all env vars set", func(t *testing.T) {
		os.Setenv(EnvSigstoreRootFile, certPath)
		os.Setenv(EnvSigstoreRekorPublicKey, pubPath)
		os.Setenv(EnvSigstoreCTLogPublicKeyFile, pubPath)
		defer os.Unsetenv(EnvSigstoreRootFile)
		defer os.Unsetenv(EnvSigstoreRekorPublicKey)
		defer os.Unsetenv(EnvSigstoreCTLogPublicKeyFile)

		material, err := getTrustedMaterial("http://localhost")
		assert.NoError(t, err)
		assert.NotNil(t, material)
	})
}
