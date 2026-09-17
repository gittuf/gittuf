package sigstore

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestGetTrustedMaterial(t *testing.T) {
	// Without env vars, it should just return the default trusted root.
	os.Unsetenv(EnvSigstoreRootFile)
	os.Unsetenv(EnvSigstoreCTLogPublicKeyFile)
	os.Unsetenv(EnvSigstoreRekorPublicKey)

	_, err := getTrustedMaterial("")
	assert.NoError(t, err)

	// Set invalid env var, it should fail
	os.Setenv(EnvSigstoreRootFile, "non_existent_file.pem")
	defer os.Unsetenv(EnvSigstoreRootFile)

	_, err = getTrustedMaterial("")
	assert.Error(t, err)
}
