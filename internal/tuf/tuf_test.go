package tuf

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLoadKeyFromBytes(t *testing.T) {
	publicKeyPath := filepath.Join("test-data", "test-key.pub")
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	key, err := LoadKeyFromBytes(publicKeyBytes)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "52e3b8e73279d6ebdd62a5016e2725ff284f569665eb92ccb145d83817a02997", key.ID())
	assert.Equal(t, "3f586ce67329419fb0081bd995914e866a7205da463d593b3b490eab2b27fd3f", key.KeyVal.Public)
}
