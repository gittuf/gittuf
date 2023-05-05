package ed25519

import (
	"context"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	e "crypto/ed25519"

	"github.com/adityasaky/gittuf/internal/signerverifier/common"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestNewSignerVerifierFromSecureSystemsLibFormat(t *testing.T) {
	t.Run("test private key", func(t *testing.T) {
		privateKeyPath := filepath.Join("test-data", "test-key")
		privateKeyBytes, err := os.ReadFile(privateKeyPath)
		if err != nil {
			t.Fatal(err)
		}

		sv, err := NewSignerVerifierFromSecureSystemsLibFormat(privateKeyBytes)
		if err != nil {
			t.Error(err)
		}

		expectedPrivateString := "66f6ebad4aeb949b91c84c9cfd6ee351fc4fd544744bab6e30fb400ba13c6e9a3f586ce67329419fb0081bd995914e866a7205da463d593b3b490eab2b27fd3f"
		expectedPrivateKey := e.PrivateKey(hexDecode(t, expectedPrivateString))

		expectedPublicString := "3f586ce67329419fb0081bd995914e866a7205da463d593b3b490eab2b27fd3f"
		expectedPublicKey := e.PublicKey(hexDecode(t, expectedPublicString))

		assert.Equal(t, "52e3b8e73279d6ebdd62a5016e2725ff284f569665eb92ccb145d83817a02997", sv.keyID)
		assert.Equal(t, expectedPrivateKey, sv.private)
		assert.Equal(t, expectedPublicKey, sv.public)
	})

	t.Run("test public key", func(t *testing.T) {
		publicKeyPath := filepath.Join("test-data", "test-key.pub")
		publicKeyBytes, err := os.ReadFile(publicKeyPath)
		if err != nil {
			t.Fatal(err)
		}

		sv, err := NewSignerVerifierFromSecureSystemsLibFormat(publicKeyBytes)
		if err != nil {
			t.Error(err)
		}

		expectedPublicString := "3f586ce67329419fb0081bd995914e866a7205da463d593b3b490eab2b27fd3f"
		expectedPublicKey := e.PublicKey(hexDecode(t, expectedPublicString))

		assert.Equal(t, "52e3b8e73279d6ebdd62a5016e2725ff284f569665eb92ccb145d83817a02997", sv.keyID)
		assert.Equal(t, expectedPublicKey, sv.public)
		assert.Nil(t, sv.private)
	})

}

func TestNewSignerVerifierFromTUFKey(t *testing.T) {
	publicKeyPath := filepath.Join("test-data", "test-key.pub")
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(publicKeyBytes)
	if err != nil {
		t.Error(err)
	}

	sv, err := NewSignerVerifierFromTUFKey(key)
	if err != nil {
		t.Error(err)
	}

	expectedPublicString := "3f586ce67329419fb0081bd995914e866a7205da463d593b3b490eab2b27fd3f"
	expectedPublicKey := e.PublicKey(hexDecode(t, expectedPublicString))

	assert.Equal(t, "52e3b8e73279d6ebdd62a5016e2725ff284f569665eb92ccb145d83817a02997", sv.keyID)
	assert.Equal(t, expectedPublicKey, sv.public)
	assert.Nil(t, sv.private)
}

func TestEd25519SignerVeriferSign(t *testing.T) {
	privateKeyPath := filepath.Join("test-data", "test-key")
	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	sv, err := NewSignerVerifierFromSecureSystemsLibFormat(privateKeyBytes)
	if err != nil {
		t.Error(err)
	}

	message := []byte("test message")

	signature, err := sv.Sign(context.Background(), message)
	if err != nil {
		t.Error(err)
	}

	expectedSignature := []byte{0x80, 0x72, 0xb4, 0x31, 0xc5, 0xa3, 0x7e, 0xc, 0xf3, 0x91, 0x22, 0x3, 0x60, 0xbf, 0x92, 0xa4, 0x46, 0x31, 0x84, 0x83, 0xf1, 0x31, 0x3, 0xdc, 0xbc, 0x5, 0x6f, 0xab, 0x84, 0xe4, 0xdc, 0xe9, 0xf5, 0x1c, 0xa9, 0xb3, 0x95, 0xa5, 0xa0, 0x16, 0xd3, 0xaa, 0x4d, 0xe7, 0xde, 0xaf, 0xc2, 0x5e, 0x1e, 0x9a, 0x9d, 0xc8, 0xb2, 0x5c, 0x1c, 0x68, 0xf7, 0x28, 0xb4, 0x1, 0x4d, 0x9f, 0xc8, 0x4}
	assert.Equal(t, expectedSignature, signature)

	publicKeyPath := filepath.Join("test-data", "test-key.pub")
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	sv, err = NewSignerVerifierFromSecureSystemsLibFormat(publicKeyBytes)
	if err != nil {
		t.Error(err)
	}

	_, err = sv.Sign(context.Background(), message)
	assert.ErrorIs(t, err, common.ErrNotPrivateKey)
}

func TestEd25519SignerVerifierVerify(t *testing.T) {
	publicKeyPath := filepath.Join("test-data", "test-key.pub")
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	sv, err := NewSignerVerifierFromSecureSystemsLibFormat(publicKeyBytes)
	if err != nil {
		t.Error(err)
	}

	message := []byte("test message")
	signature := []byte{0x80, 0x72, 0xb4, 0x31, 0xc5, 0xa3, 0x7e, 0xc, 0xf3, 0x91, 0x22, 0x3, 0x60, 0xbf, 0x92, 0xa4, 0x46, 0x31, 0x84, 0x83, 0xf1, 0x31, 0x3, 0xdc, 0xbc, 0x5, 0x6f, 0xab, 0x84, 0xe4, 0xdc, 0xe9, 0xf5, 0x1c, 0xa9, 0xb3, 0x95, 0xa5, 0xa0, 0x16, 0xd3, 0xaa, 0x4d, 0xe7, 0xde, 0xaf, 0xc2, 0x5e, 0x1e, 0x9a, 0x9d, 0xc8, 0xb2, 0x5c, 0x1c, 0x68, 0xf7, 0x28, 0xb4, 0x1, 0x4d, 0x9f, 0xc8, 0x4}
	assert.Nil(t, sv.Verify(context.Background(), message, signature))

	message = []byte("corrupted message")
	err = sv.Verify(context.Background(), message, signature)
	assert.ErrorIs(t, err, common.ErrSignatureVerificationFailed)
}

func hexDecode(t *testing.T, data string) []byte {
	t.Helper()
	b, err := hex.DecodeString(data)
	if err != nil {
		t.Fatal(err)
	}
	return b
}
