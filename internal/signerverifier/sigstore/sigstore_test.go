// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sigstore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generateTestSigner() (*x509.Certificate, *ecdsa.PrivateKey, error) {
	log.Println("Starting generateTestSigner...")

	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		log.Printf("Failed to generate ECDSA key: %v", err)
		return nil, nil, err
	}
	log.Println("ECDSA key pair generated.")

	notBefore := time.Now()
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumber, err := rand.Int(rand.Reader, new(big.Int).Lsh(big.NewInt(1), 128))
	if err != nil {
		log.Printf("Failed to generate serial number: %v", err)
		return nil, nil, err
	}
	log.Printf("Generated serial number: %v", serialNumber)

	subjectKeyID := generateSubjectKeyID(&priv.PublicKey)
	log.Printf("Using SubjectKeyId: %s", hex.EncodeToString(subjectKeyID))

	template := x509.Certificate{
		SerialNumber: serialNumber,
		Subject: pkix.Name{
			Organization: []string{"Test TSA"},
			CommonName:   "Test TSA Root",
		},
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping},
		IsCA:                  true,
		BasicConstraintsValid: true,
		SubjectKeyId:          subjectKeyID,
	}

	log.Println("Creating self-signed TSA CA certificate...")
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		log.Printf("Failed to create CA certificate: %v", err)
		return nil, nil, err
	}
	log.Println("Self-signed TSA CA certificate created successfully.")

	cert, err := x509.ParseCertificate(certDER)
	if err != nil {
		log.Printf("Failed to parse CA certificate: %v", err)
		return nil, nil, err
	}
	log.Println("Parsed CA certificate successfully.")

	return cert, priv, nil
}

func generateSubjectKeyID(pub *ecdsa.PublicKey) []byte {
	pubBytes, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		log.Printf("Failed to marshal public key: %v", err)
		return nil
	}
	hash := sha256.Sum256(pubBytes)
	return hash[:]
}

func generateTrustedRootFile(filePath string) error {
	caCert, _, err := generateTestSigner()
	if err != nil {
		return fmt.Errorf("failed to generate test signer: %w", err)
	}

	trustedRoot := map[string]any{
		"mediaType": "application/vnd.dev.sigstore.trustedroot+json;version=0.1",
		"tlogs": []map[string]any{
			{
				"baseUrl":       "https://rekor.sigstore.dev",
				"hashAlgorithm": "SHA2_256",
				"publicKey": map[string]any{
					"rawBytes":   "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE2G2Y+2tabdTV5BcGiBIx0a9fAFwrkBbmLSGtks4L3qX6yYY0zufBnhC8Ur/iy55GhWP/9A/bY2LhC30M9+RYtw==",
					"keyDetails": "PKIX_ECDSA_P256_SHA_256",
					"validFor": map[string]any{
						"start": "2021-01-12T11:53:27Z",
					},
				},
				"logId": map[string]any{
					"keyId": "wNI9atQGlz+VWfO6LRygH4QUfY/8W4RFwiT5i5WRgB0=",
				},
			},
		},
		"certificateAuthorities": []map[string]any{
			{
				"subject": map[string]any{
					"organization": "sigstore.dev",
					"commonName":   "sigstore",
				},
				"uri": "https://fulcio.sigstore.dev",
				"certChain": map[string]any{
					"certificates": []map[string]any{
						{
							"rawBytes": "MIIB+DCCAX6gAwIBAgITNVkDZoCiofPDsy7dfm6geLbuhzAKBggqhkjOPQQDAzAqMRUwEwYDVQQKEwxzaWdzdG9yZS5kZXYxETAPBgNVBAMTCHNpZ3N0b3JlMB4XDTIxMDMwNzAzMjAyOVoXDTMxMDIyMzAzMjAyOVowKjEVMBMGA1UEChMMc2lnc3RvcmUuZGV2MREwDwYDVQQDEwhzaWdzdG9yZTB2MBAGByqGSM49AgEGBSuBBAAiA2IABLSyA7Ii5k+pNO8ZEWY0ylemWDowOkNa3kL+GZE5Z5GWehL9/A9bRNA3RbrsZ5i0JcastaRL7Sp5fp/jD5dxqc/UdTVnlvS16an+2Yfswe/QuLolRUCrcOE2+2iA5+tzd6NmMGQwDgYDVR0PAQH/BAQDAgEGMBIGA1UdEwEB/wQIMAYBAf8CAQEwHQYDVR0OBBYEFMjFHQBBmiQpMlEk6w2uSu1KBtPsMB8GA1UdIwQYMBaAFMjFHQBBmiQpMlEk6w2uSu1KBtPsMAoGCCqGSM49BAMDA2gAMGUCMH8liWJfMui6vXXBhjDgY4MwslmN/TJxVe/83WrFomwmNf056y1X48F9c4m3a3ozXAIxAKjRay5/aj/jsKKGIkmQatjI8uupHr/+CxFvaJWmpYqNkLDGRU+9orzh5hI2RrcuaQ==",
						},
					},
				},
				"validFor": map[string]interface{}{
					"start": "2021-03-07T03:20:29Z",
					"end":   "2022-12-31T23:59:59Z",
				},
			},
		},
		"ctlogs": []map[string]any{
			{
				"baseUrl":       "https://ctfe.sigstore.dev/test",
				"hashAlgorithm": "SHA2_256",
				"publicKey": map[string]interface{}{
					"rawBytes":   "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEbfwR+RJudXscgRBRpKX1XFDy3PyudDxz/SfnRi1fT8ekpfBd2O1uoz7jr3Z8nKzxA69EUQ+eFCFI3zeubPWU7w==",
					"keyDetails": "PKIX_ECDSA_P256_SHA_256",
					"validFor": map[string]interface{}{
						"start": "2021-03-14T00:00:00Z",
						"end":   "2022-10-31T23:59:59Z",
					},
				},
				"logId": map[string]interface{}{
					"keyId": "CGCS8ChS/2hF0dFrJ4ScRWcYrBY9wzjSbea8IgY2b3I=",
				},
			},
		},
		"timestampAuthorities": []map[string]any{
			{
				"subject": map[string]interface{}{
					"organization": "Test TSA",
					"commonName":   "Test TSA Root",
				},
				"uri": "https://tsa.example.com",
				"certChain": map[string]interface{}{
					"certificates": []map[string]interface{}{
						{
							"rawBytes": encodeToBase64(caCert),
						},
					},
				},
				"validFor": map[string]interface{}{
					"start": "2024-01-01T00:00:00Z",
					"end":   "2025-01-01T00:00:00Z",
				},
			},
		},
	}

	data, err := json.MarshalIndent(trustedRoot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trusted root: %w", err)
	}

	log.Printf("Generated root.json content:\n%s", string(data))

	err = os.WriteFile(filePath, data, 0600)
	if err != nil {
		return fmt.Errorf("failed to write root.json: %w", err)
	}
	return nil
}

func encodeToBase64(cert *x509.Certificate) string {
	return base64.StdEncoding.EncodeToString(cert.Raw)
}

func TestGetTUFRootInterfaceMethods(t *testing.T) {
	tmpDir := t.TempDir()

	originalHome := os.Getenv("HOME")
	os.Setenv("HOME", tmpDir)
	defer os.Setenv("HOME", originalHome)

	sigstoreDir := filepath.Join(tmpDir, ".sigstore", "root")
	err := os.MkdirAll(sigstoreDir, 0755)
	require.NoError(t, err, "Failed to create temporary sigstore directory")

	trustedRootPath := filepath.Join(sigstoreDir, "root.json")
	err = generateTrustedRootFile(trustedRootPath)
	require.NoError(t, err, "Failed to generate trusted root file")

	t.Logf("Trusted root file path: %s", trustedRootPath)
	if _, err := os.Stat(trustedRootPath); os.IsNotExist(err) {
		t.Fatalf("Trusted root file does not exist at %s", trustedRootPath)
	}
	fileContent, err := os.ReadFile(trustedRootPath)
	if err != nil {
		t.Fatalf("Failed to read trusted root file: %v", err)
	}
	t.Logf("Trusted root file content:\n%s", string(fileContent))

	v := &Verifier{}

	trustedRoot, err := v.getTUFRoot()
	require.NoError(t, err, "getTUFRoot should succeed with valid root.json")
	require.NotNil(t, trustedRoot, "getTUFRoot should return a non-nil TrustedMaterial")

	t.Run("FulcioCertificateAuthorities", func(t *testing.T) {
		certAuthorities := trustedRoot.FulcioCertificateAuthorities()
		assert.NotEmpty(t, certAuthorities, "FulcioCertificateAuthorities should return at least one CA")

		if len(certAuthorities) > 0 {
			ca := certAuthorities[0]
			verifiedCa, err := ca.Verify(nil, time.Now())
			assert.Error(t, err, "CA verification should fail without a certificate")
			assert.Nil(t, verifiedCa, "CA verification should return nil without a certificate")
		}
	})

	t.Run("RekorLogs", func(t *testing.T) {
		rekorLogs := trustedRoot.RekorLogs()
		assert.NotEmpty(t, rekorLogs, "RekorLogs should return at least one log")

		for logID, log := range rekorLogs {
			assert.NotEmpty(t, logID, "Log ID should not be empty")
			assert.NotEmpty(t, log.BaseURL, "Log should have a base URL")
			assert.NotNil(t, log.PublicKey, "Log should have a public key")
			assert.NotEmpty(t, log.HashFunc, "Log should have a hash algorithm")
		}
	})

	t.Run("CTLogs", func(t *testing.T) {
		ctLogs := trustedRoot.CTLogs()
		assert.NotEmpty(t, ctLogs, "CTLogs should return at least one log")

		for logID, log := range ctLogs {
			assert.NotEmpty(t, logID, "Log ID should not be empty")
			assert.NotEmpty(t, log.BaseURL, "Log should have a base URL")
			assert.NotNil(t, log.PublicKey, "Log should have a public key")
			assert.NotEmpty(t, log.HashFunc, "Log should have a hash algorithm")
		}
	})
}
