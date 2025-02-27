package sigstore

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"math/big"
	"os"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"crypto/sha1"
	"encoding/hex"
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
		NotBefore: notBefore,
		NotAfter:  notAfter,

		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageTimeStamping}, // Required for TSA
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
	hash := sha1.Sum(pubBytes)
	return hash[:]
}

func generateTrustedRootFile(filePath string) (x509.Certificate, ecdsa.PrivateKey, error) {
	caCert, caPriv, err := generateTestSigner()
	if err != nil {
		log.Fatal("Failed to generate TSA Root:", err)
	}

	trustedRoot := map[string]interface{}{
		"mediaType": "application/vnd.dev.sigstore.trustedroot+json;version=0.1",
		"tlogs": []map[string]interface{}{
			{
				"baseUrl":       "https://rekor.sigstore.dev",
				"hashAlgorithm": "SHA2_256",
				"publicKey": map[string]interface{}{
					"rawBytes":   "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE2G2Y+2tabdTV5BcGiBIx0a9fAFwrkBbmLSGtks4L3qX6yYY0zufBnhC8Ur/iy55GhWP/9A/bY2LhC30M9+RYtw==",
					"keyDetails": "PKIX_ECDSA_P256_SHA_256",
					"validFor": map[string]interface{}{
						"start": "2021-01-12T11:53:27.000Z",
					},
				},
				"logId": map[string]interface{}{
					"keyId": "wNI9atQGlz+VWfO6LRygH4QUfY/8W4RFwiT5i5WRgB0=",
				},
			},
		},
		"certificateAuthorities": []map[string]interface{}{
			{
				"subject": map[string]interface{}{
					"organization": "sigstore.dev",
					"commonName":   "sigstore",
				},
				"uri": "https://fulcio.sigstore.dev",
				"certChain": map[string]interface{}{
					"certificates": []map[string]interface{}{
						{
							"rawBytes": "MIIB+DCCAX6gAwIBAgITNVkDZoCiofPDsy7dfm6geLbuhzAKBggqhkjOPQQDAzAqMRUwEwYDVQQKEwxzaWdzdG9yZS5kZXYxETAPBgNVBAMTCHNpZ3N0b3JlMB4XDTIxMDMwNzAzMjAyOVoXDTMxMDIyMzAzMjAyOVowKjEVMBMGA1UEChMMc2lnc3RvcmUuZGV2MREwDwYDVQQDEwhzaWdzdG9yZTB2MBAGByqGSM49AgEGBSuBBAAiA2IABLSyA7Ii5k+pNO8ZEWY0ylemWDowOkNa3kL+GZE5Z5GWehL9/A9bRNA3RbrsZ5i0JcastaRL7Sp5fp/jD5dxqc/UdTVnlvS16an+2Yfswe/QuLolRUCrcOE2+2iA5+tzd6NmMGQwDgYDVR0PAQH/BAQDAgEGMBIGA1UdEwEB/wQIMAYBAf8CAQEwHQYDVR0OBBYEFMjFHQBBmiQpMlEk6w2uSu1KBtPsMB8GA1UdIwQYMBaAFMjFHQBBmiQpMlEk6w2uSu1KBtPsMAoGCCqGSM49BAMDA2gAMGUCMH8liWJfMui6vXXBhjDgY4MwslmN/TJxVe/83WrFomwmNf056y1X48F9c4m3a3ozXAIxAKjRay5/aj/jsKKGIkmQatjI8uupHr/+CxFvaJWmpYqNkLDGRU+9orzh5hI2RrcuaQ==",
						},
					},
				},
				"validFor": map[string]interface{}{
					"start": "2021-03-07T03:20:29.000Z",
					"end":   "2022-12-31T23:59:59.999Z",
				},
			},
			{
				"subject": map[string]interface{}{
					"organization": "sigstore.dev",
					"commonName":   "sigstore",
				},
				"uri": "https://fulcio.sigstore.dev",
				"certChain": map[string]interface{}{
					"certificates": []map[string]interface{}{
						{
							"rawBytes": "MIICGjCCAaGgAwIBAgIUALnViVfnU0brJasmRkHrn/UnfaQwCgYIKoZIzj0EAwMwKjEVMBMGA1UEChMMc2lnc3RvcmUuZGV2MREwDwYDVQQDEwhzaWdzdG9yZTAeFw0yMjA0MTMyMDA2MTVaFw0zMTEwMDUxMzU2NThaMDcxFTATBgNVBAoTDHNpZ3N0b3JlLmRldjEeMBwGA1UEAxMVc2lnc3RvcmUtaW50ZXJtZWRpYXRlMHYwEAYHKoZIzj0CAQYFK4EEACIDYgAE8RVS/ysH+NOvuDZyPIZtilgUF9NlarYpAd9HP1vBBH1U5CV77LSS7s0ZiH4nE7Hv7ptS6LvvR/STk798LVgMzLlJ4HeIfF3tHSaexLcYpSASr1kS0N/RgBJz/9jWCiXno3sweTAOBgNVHQ8BAf8EBAMCAQYwEwYDVR0lBAwwCgYIKwYBBQUHAwMwEgYDVR0TAQH/BAgwBgEB/wIBADAdBgNVHQ4EFgQU39Ppz1YkEZb5qNjpKFWixi4YZD8wHwYDVR0jBBgwFoAUWMAeX5FFpWapesyQoZMi0CrFxfowCgYIKoZIzj0EAwMDZwAwZAIwPCsQK4DYiZYDPIaDi5HFKnfxXx6ASSVmERfsynYBiX2X6SJRnZU84/9DZdnFvvxmAjBOt6QpBlc4J/0DxvkTCqpclvziL6BCCPnjdlIB3Pu3BxsPmygUY7Ii2zbdCdliiow=",
						},
						{
							"rawBytes": "MIIB9zCCAXygAwIBAgIUALZNAPFdxHPwjeDloDwyYChAO/4wCgYIKoZIzj0EAwMwKjEVMBMGA1UEChMMc2lnc3RvcmUuZGV2MREwDwYDVQQDEwhzaWdzdG9yZTAeFw0yMTEwMDcxMzU2NTlaFw0zMTEwMDUxMzU2NThaMCoxFTATBgNVBAoTDHNpZ3N0b3JlLmRldjERMA8GA1UEAxMIc2lnc3RvcmUwdjAQBgcqhkjOPQIBBgUrgQQAIgNiAAT7XeFT4rb3PQGwS4IajtLk3/OlnpgangaBclYpsYBr5i+4ynB07ceb3LP0OIOZdxexX69c5iVuyJRQ+Hz05yi+UF3uBWAlHpiS5sh0+H2GHE7SXrk1EC5m1Tr19L9gg92jYzBhMA4GA1UdDwEB/wQEAwIBBjAPBgNVHRMBAf8EBTADAQH/MB0GA1UdDgQWBBRYwB5fkUWlZql6zJChkyLQKsXF+jAfBgNVHSMEGDAWgBRYwB5fkUWlZql6zJChkyLQKsXF+jAKBggqhkjOPQQDAwNpADBmAjEAj1nHeXZp+13NWBNa+EDsDP8G1WWg1tCMWP/WHPqpaVo0jhsweNFZgSs0eE7wYI4qAjEA2WB9ot98sIkoF3vZYdd3/VtWB5b9TNMea7Ix/stJ5TfcLLeABLE4BNJOsQ4vnBHJ",
						},
					},
				},
				"validFor": map[string]interface{}{
					"start": "2022-04-13T20:06:15.000Z",
				},
			},
		},
		"ctlogs": []map[string]interface{}{
			{
				"baseUrl":       "https://ctfe.sigstore.dev/test",
				"hashAlgorithm": "SHA2_256",
				"publicKey": map[string]interface{}{
					"rawBytes":   "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEbfwR+RJudXscgRBRpKX1XFDy3PyudDxz/SfnRi1fT8ekpfBd2O1uoz7jr3Z8nKzxA69EUQ+eFCFI3zeubPWU7w==",
					"keyDetails": "PKIX_ECDSA_P256_SHA_256",
					"validFor": map[string]interface{}{
						"start": "2021-03-14T00:00:00.000Z",
						"end":   "2022-10-31T23:59:59.999Z",
					},
				},
				"logId": map[string]interface{}{
					"keyId": "CGCS8ChS/2hF0dFrJ4ScRWcYrBY9wzjSbea8IgY2b3I=",
				},
			},
			{
				"baseUrl":       "https://ctfe.sigstore.dev/2022",
				"hashAlgorithm": "SHA2_256",
				"publicKey": map[string]interface{}{
					"rawBytes":   "MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEiPSlFi0CmFTfEjCUqF9HuCEcYXNKAaYalIJmBZ8yyezPjTqhxrKBpMnaocVtLJBI1eM3uXnQzQGAJdJ4gs9Fyw==",
					"keyDetails": "PKIX_ECDSA_P256_SHA_256",
					"validFor": map[string]interface{}{
						"start": "2022-10-20T00:00:00.000Z",
					},
				},
				"logId": map[string]interface{}{
					"keyId": "3T0wasbHETJjGR4cmWc3AqJKXrjePK3/h4pygC8p7o4=",
				},
			},
		},
		"timestamp_authorities": []map[string]interface{}{
			{
				"subject": map[string]string{
					"organization": "Test TSA",
					"commonName":   "Test TSA Root",
				},
				"uri": "https://tsa.example.com",
				"certChain": map[string]interface{}{
					"certificates": []map[string]string{
						{"rawBytes": encodeToBase64(caCert)},
					},
				},
				"validFor": map[string]string{
					"start": "2024-01-01T00:00:00Z",
				},
			},
		},
	}

	data, err := json.MarshalIndent(trustedRoot, "", "  ")
	if err != nil {
		return *caCert, *caPriv, fmt.Errorf("Failed to marshal trusted root: %v", err)
	}
	return *caCert, *caPriv, os.WriteFile(filePath, data, 0644)
}

func encodeToBase64(cert *x509.Certificate) string {
	return base64.StdEncoding.EncodeToString(cert.Raw)
}

func TestGetTUFRootInterfaceMethods(t *testing.T) {
	tmpDir := t.TempDir()
	trustedRootPath := tmpDir + "/trusted_root.json"
	_, _, err := generateTrustedRootFile(trustedRootPath)
	if err != nil {
		t.Fatalf("Failed to generate trusted root: %v", err)
	}

	os.Setenv("SIGSTORE_ROOT_FILE", trustedRootPath)

	v := &Verifier{}

	trustedRoot, err := v.getTUFRoot()
	require.NoError(t, err, "getTUFRoot should succeed with valid trusted_root.json")
	require.NotNil(t, trustedRoot, "getTUFRoot should return a non-nil TrustedMaterial")

	t.Run("TimestampingAuthorities", func(t *testing.T) {
		tsa := trustedRoot.TimestampingAuthorities()
		assert.Equal(t, 1, len(tsa), "TimestampingAuthorities should return one TSA")
	})

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

	t.Run("Error_MissingEnvVar", func(t *testing.T) {
		os.Unsetenv(EnvSigstoreRootFile)

		_, err := v.getTUFRoot()
		assert.Error(t, err, "getTUFRoot should fail when environment variable is not set")
		assert.Contains(t, err.Error(), tuf.ErrSigstoreTrustedRootNotSet.Error(),
			"Error message should mention the missing environment variable")
	})

	t.Run("Error_FileNotFound", func(t *testing.T) {
		os.Setenv(EnvSigstoreRootFile, "/path/to/nonexistent/file.json")

		_, err := v.getTUFRoot()
		assert.Error(t, err, "getTUFRoot should fail when file doesn't exist")
		assert.Contains(t, err.Error(), "error reading trusted_root.json",
			"Error message should mention reading error")
	})

	t.Run("Error_InvalidJSON", func(t *testing.T) {
		tmpfile, err := os.CreateTemp("", "invalid_root_*.json")
		require.NoError(t, err)
		defer os.Remove(tmpfile.Name())

		_, err = tmpfile.Write([]byte(`{invalid json}`))
		require.NoError(t, err)
		tmpfile.Close()

		os.Setenv(EnvSigstoreRootFile, tmpfile.Name())

		_, err = v.getTUFRoot()
		assert.Error(t, err, "getTUFRoot should fail when JSON is invalid")
		assert.Contains(t, err.Error(), "error parsing trusted_root.json",
			"Error message should mention parsing error")
	})
}
