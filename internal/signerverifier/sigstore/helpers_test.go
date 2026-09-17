package sigstore

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestParseTokenForIdentityAndIssuer(t *testing.T) {
	_, _, err := parseTokenForIdentityAndIssuer("invalid.token", "")
	assert.Error(t, err)
}

func TestParsePEMFile(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.pem")
	_, err := parsePEMFile(path)
	assert.Error(t, err)

	err = os.WriteFile(path, []byte("invalid pem"), 0600)
	assert.NoError(t, err)
	_, err = parsePEMFile(path)
	assert.Error(t, err)
}

func TestParsePubKey(t *testing.T) {
	tmpDir := t.TempDir()
	path := filepath.Join(tmpDir, "test.pub")
	_, _, err := parsePubKey(path)
	assert.Error(t, err)

	err = os.WriteFile(path, []byte("invalid pub"), 0600)
	assert.NoError(t, err)
	_, _, err = parsePubKey(path)
	assert.Error(t, err)
}
