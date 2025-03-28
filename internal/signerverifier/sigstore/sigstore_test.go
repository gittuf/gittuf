package sigstore

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func generatePublicTUFRootFile(filePath string) error {
	tufRoot := map[string]interface{}{
		"signatures": []map[string]interface{}{
			{
				"keyid": "6f260089d5923daf20166ca657c543af618346ab971884a99962b01988bbe0c3",
				"sig":   "",
			},
			{
				"keyid": "e71a54d543835ba86adad9460379c7641fb8726d164ea766801a1c522aba7ea2",
				"sig":   "3045022100b0bcf189ce1b93e7db9649d5be512a1880c0e358870e3933e426c5afb8a4061002206d214bd79b09f458ccc521a290aa960c417014fc16e606f82091b5e31814886a",
			},
			{
				"keyid": "22f4caec6d8e6f9555af66b3d4c3cb06a3bb23fdc7e39c916c61f462e6f52b06",
				"sig":   "",
			},
			{
				"keyid": "61643838125b440b40db6942f5cb5a31c0dc04368316eb2aaa58b95904a58222",
				"sig":   "3045022100a9b9e294ec21b62dfca6a16a19d084182c12572e33d9c4dcab5317fa1e8a459d022069f68e55ea1f95c5a367aac7a61a65757f93da5a006a5f4d1cf995be812d7602",
			},
			{
				"keyid": "a687e5bf4fab82b0ee58d46e05c9535145a2c9afb458f43d42b45ca0fdce2a70",
				"sig":   "30440220781178ec3915cb16aca757d40e28435ac5378d6b487acb111d1eeb339397f79a0220781cce48ae46f9e47b97a8414fcf466a986726a5896c72a0e4aba3162cb826dd",
			},
		},
		"signed": map[string]interface{}{
			"_type":               "root",
			"consistent_snapshot": true,
			"expires":             "2025-08-19T14:33:09Z",
			"keys": map[string]interface{}{
				"0c87432c3bf09fd99189fdc32fa5eaedf4e4a5fac7bab73fa04a2e0fc64af6f5": map[string]interface{}{
					"keyid_hash_algorithms": []string{"sha256", "sha512"},
					"keytype":               "ecdsa",
					"keyval": map[string]interface{}{
						"public": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEWRiGr5+j+3J5SsH+Ztr5nE2H2wO7\nBV+nO3s93gLca18qTOzHY1oWyAGDykMSsGTUBSt9D+An0KfKsD2mfSM42Q==\n-----END PUBLIC KEY-----\n",
					},
					"scheme":                 "ecdsa-sha2-nistp256",
					"x-tuf-on-ci-online-uri": "gcpkms:projects/sigstore-root-signing/locations/global/keyRings/root/cryptoKeys/timestamp/cryptoKeyVersions/1",
				},
				"22f4caec6d8e6f9555af66b3d4c3cb06a3bb23fdc7e39c916c61f462e6f52b06": map[string]interface{}{
					"keyid_hash_algorithms": []string{"sha256", "sha512"},
					"keytype":               "ecdsa",
					"keyval": map[string]interface{}{
						"public": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEzBzVOmHCPojMVLSI364WiiV8NPrD\n6IgRxVliskz/v+y3JER5mcVGcONliDcWMC5J2lfHmjPNPhb4H7xm8LzfSA==\n-----END PUBLIC KEY-----\n",
					},
					"scheme":               "ecdsa-sha2-nistp256",
					"x-tuf-on-ci-keyowner": "@santiagotorres",
				},
				"61643838125b440b40db6942f5cb5a31c0dc04368316eb2aaa58b95904a58222": map[string]interface{}{
					"keyid_hash_algorithms": []string{"sha256", "sha512"},
					"keytype":               "ecdsa",
					"keyval": map[string]interface{}{
						"public": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEinikSsAQmYkNeH5eYq/CnIzLaacO\nxlSaawQDOwqKy/tCqxq5xxPSJc21K4WIhs9GyOkKfzueY3GILzcMJZ4cWw==\n-----END PUBLIC KEY-----\n",
					},
					"scheme":               "ecdsa-sha2-nistp256",
					"x-tuf-on-ci-keyowner": "@bobcallaway",
				},
				"6f260089d5923daf20166ca657c543af618346ab971884a99962b01988bbe0c3": map[string]interface{}{
					"keyid_hash_algorithms": []string{"sha256", "sha512"},
					"keytype":               "ecdsa",
					"keyval": map[string]interface{}{
						"public": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEy8XKsmhBYDI8Jc0GwzBxeKax0cm5\nSTKEU65HPFunUn41sT8pi0FjM4IkHz/YUmwmLUO0Wt7lxhj6BkLIK4qYAw==\n-----END PUBLIC KEY-----\n",
					},
					"scheme":               "ecdsa-sha2-nistp256",
					"x-tuf-on-ci-keyowner": "@dlorenc",
				},
				"a687e5bf4fab82b0ee58d46e05c9535145a2c9afb458f43d42b45ca0fdce2a70": map[string]interface{}{
					"keyid_hash_algorithms": []string{"sha256", "sha512"},
					"keytype":               "ecdsa",
					"keyval": map[string]interface{}{
						"public": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAE0ghrh92Lw1Yr3idGV5WqCtMDB8Cx\n+D8hdC4w2ZLNIplVRoVGLskYa3gheMyOjiJ8kPi15aQ2//7P+oj7UvJPGw==\n-----END PUBLIC KEY-----\n",
					},
					"scheme":               "ecdsa-sha2-nistp256",
					"x-tuf-on-ci-keyowner": "@joshuagl",
				},
				"e71a54d543835ba86adad9460379c7641fb8726d164ea766801a1c522aba7ea2": map[string]interface{}{
					"keyid_hash_algorithms": []string{"sha256", "sha512"},
					"keytype":               "ecdsa",
					"keyval": map[string]interface{}{
						"public": "-----BEGIN PUBLIC KEY-----\nMFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEEXsz3SZXFb8jMV42j6pJlyjbjR8K\nN3Bwocexq6LMIb5qsWKOQvLN16NUefLc4HswOoumRsVVaajSpQS6fobkRw==\n-----END PUBLIC KEY-----\n",
					},
					"scheme":               "ecdsa-sha2-nistp256",
					"x-tuf-on-ci-keyowner": "@mnm678",
				},
			},
			"roles": map[string]interface{}{
				"root": map[string]interface{}{
					"keyids": []string{
						"6f260089d5923daf20166ca657c543af618346ab971884a99962b01988bbe0c3",
						"e71a54d543835ba86adad9460379c7641fb8726d164ea766801a1c522aba7ea2",
						"22f4caec6d8e6f9555af66b3d4c3cb06a3bb23fdc7e39c916c61f462e6f52b06",
						"61643838125b440b40db6942f5cb5a31c0dc04368316eb2aaa58b95904a58222",
						"a687e5bf4fab82b0ee58d46e05c9535145a2c9afb458f43d42b45ca0fdce2a70",
					},
					"threshold": 3,
				},
				"snapshot": map[string]interface{}{
					"keyids":                     []string{"0c87432c3bf09fd99189fdc32fa5eaedf4e4a5fac7bab73fa04a2e0fc64af6f5"},
					"threshold":                  1,
					"x-tuf-on-ci-expiry-period":  3650,
					"x-tuf-on-ci-signing-period": 365,
				},
				"targets": map[string]interface{}{
					"keyids": []string{
						"6f260089d5923daf20166ca657c543af618346ab971884a99962b01988bbe0c3",
						"e71a54d543835ba86adad9460379c7641fb8726d164ea766801a1c522aba7ea2",
						"22f4caec6d8e6f9555af66b3d4c3cb06a3bb23fdc7e39c916c61f462e6f52b06",
						"61643838125b440b40db6942f5cb5a31c0dc04368316eb2aaa58b95904a58222",
						"a687e5bf4fab82b0ee58d46e05c9535145a2c9afb458f43d42b45ca0fdce2a70",
					},
					"threshold": 3,
				},
				"timestamp": map[string]interface{}{
					"keyids":                     []string{"0c87432c3bf09fd99189fdc32fa5eaedf4e4a5fac7bab73fa04a2e0fc64af6f5"},
					"threshold":                  1,
					"x-tuf-on-ci-expiry-period":  7,
					"x-tuf-on-ci-signing-period": 6,
				},
			},
			"spec_version":               "1.0",
			"version":                    12,
			"x-tuf-on-ci-expiry-period":  197,
			"x-tuf-on-ci-signing-period": 46,
		},
	}

	data, err := json.MarshalIndent(tufRoot, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal TUF root: %w", err)
	}

	return os.WriteFile(filePath, data, 0600)
}

func TestGetTUFRoot(t *testing.T) {
	tmpDir := t.TempDir()
	originalRootEnv := os.Getenv("SIGSTORE_TRUSTED_ROOT")
	defer os.Setenv("SIGSTORE_TRUSTED_ROOT", originalRootEnv)

	t.Run("DefaultPublicInstance", func(t *testing.T) {
		os.Unsetenv("SIGSTORE_TRUSTED_ROOT")
		t.Logf("Testing default public instance")

		v := &Verifier{}
		trustedRoot, err := v.getTUFRoot()
		require.NoError(t, err, "getTUFRoot should succeed with default public instance")
		require.NotNil(t, trustedRoot, "TrustedMaterial should not be nil")

		t.Run("FulcioCertificateAuthorities", func(t *testing.T) {
			certAuthorities := trustedRoot.FulcioCertificateAuthorities()
			t.Logf("Found %d Fulcio certificate authorities", len(certAuthorities))
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
			t.Logf("Found %d Rekor logs", len(rekorLogs))
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
			t.Logf("Found %d CT logs", len(ctLogs))
			assert.NotEmpty(t, ctLogs, "CTLogs should return at least one log")

			for logID, log := range ctLogs {
				assert.NotEmpty(t, logID, "Log ID should not be empty")
				assert.NotEmpty(t, log.BaseURL, "Log should have a base URL")
				assert.NotNil(t, log.PublicKey, "Log should have a public key")
				assert.NotEmpty(t, log.HashFunc, "Log should have a hash algorithm")
			}
		})
	})

	t.Run("CustomTUFRootValid", func(t *testing.T) {
		validRootPath := filepath.Join(tmpDir, "public_root.json")
		err := generatePublicTUFRootFile(validRootPath)
		require.NoError(t, err, "Failed to generate public TUF root file")

		t.Logf("Setting SIGSTORE_TRUSTED_ROOT to %s", validRootPath)
		os.Setenv("SIGSTORE_TRUSTED_ROOT", validRootPath)

		v := &Verifier{}
		trustedRoot, err := v.getTUFRoot()
		require.NoError(t, err, "getTUFRoot should succeed with valid public root.json")
		require.NotNil(t, trustedRoot, "TrustedMaterial should not be nil")

		t.Run("FulcioCertificateAuthorities", func(t *testing.T) {
			certAuthorities := trustedRoot.FulcioCertificateAuthorities()
			t.Logf("Found %d Fulcio certificate authorities", len(certAuthorities))
			assert.NotEmpty(t, certAuthorities, "FulcioCertificateAuthorities should return at least one CA")

			ca := certAuthorities[0]
			verifiedCa, err := ca.Verify(nil, time.Now())
			assert.Error(t, err, "CA verification should fail without a certificate")
			assert.Nil(t, verifiedCa, "CA verification should return nil without a certificate")
		})

		t.Run("RekorLogs", func(t *testing.T) {
			rekorLogs := trustedRoot.RekorLogs()
			t.Logf("Found %d Rekor logs", len(rekorLogs))
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
			t.Logf("Found %d CT logs", len(ctLogs))
			assert.NotEmpty(t, ctLogs, "CTLogs should return at least one log")

			for logID, log := range ctLogs {
				assert.NotEmpty(t, logID, "Log ID should not be empty")
				assert.NotEmpty(t, log.BaseURL, "Log should have a base URL")
				assert.NotNil(t, log.PublicKey, "Log should have a public key")
				assert.NotEmpty(t, log.HashFunc, "Log should have a hash algorithm")
			}
		})
	})

	t.Run("CustomTUFRootFileNotFound", func(t *testing.T) {
		nonExistentPath := filepath.Join(tmpDir, "non_existent_root.json")
		t.Logf("Setting SIGSTORE_TRUSTED_ROOT to %s (does not exist)", nonExistentPath)
		os.Setenv("SIGSTORE_TRUSTED_ROOT", nonExistentPath)

		v := &Verifier{}
		trustedRoot, err := v.getTUFRoot()
		assert.Error(t, err, "getTUFRoot should fail when file does not exist")
		assert.Nil(t, trustedRoot, "TrustedMaterial should be nil on error")
		assert.Contains(t, err.Error(), "failed to read TUF root file", "Error should indicate file read failure")
	})

	t.Run("CustomTUFRootInvalidJSON", func(t *testing.T) {
		invalidRootPath := filepath.Join(tmpDir, "invalid_root.json")
		err := os.WriteFile(invalidRootPath, []byte("invalid json"), 0600)
		require.NoError(t, err, "Failed to write invalid TUF root file")

		t.Logf("Setting SIGSTORE_TRUSTED_ROOT to %s", invalidRootPath)
		os.Setenv("SIGSTORE_TRUSTED_ROOT", invalidRootPath)

		v := &Verifier{}
		trustedRoot, err := v.getTUFRoot()
		assert.Error(t, err, "getTUFRoot should fail with invalid JSON")
		assert.Nil(t, trustedRoot, "TrustedMaterial should be nil on error")
		assert.Contains(t, err.Error(), "failed to initialize TUF client", "Error should indicate TUF client initialization failure")
	})
}
