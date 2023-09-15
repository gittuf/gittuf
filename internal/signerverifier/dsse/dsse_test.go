package dsse

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

func TestCreateEnvelope(t *testing.T) {
	rootMetadata := tuf.NewRootMetadata()
	env, err := CreateEnvelope(rootMetadata)
	assert.Nil(t, err)
	assert.Equal(t, PayloadType, env.PayloadType)
	assert.Equal(t, "eyJ0eXBlIjoicm9vdCIsInNwZWNfdmVyc2lvbiI6IjEuMCIsImNvbnNpc3RlbnRfc25hcHNob3QiOnRydWUsInZlcnNpb24iOjAsImV4cGlyZXMiOiIiLCJrZXlzIjpudWxsLCJyb2xlcyI6bnVsbH0=", env.Payload)
}

func TestSignEnvelope(t *testing.T) {
	env, err := createSignedEnvelope()
	if err != nil {
		t.Fatal(err)
	}

	assert.NotEmpty(t, env.Signatures)
	assert.Equal(t, "a0xAMWnJ3Hzf8j2zLFmniyUxV58m2lUprgzDPkJIRUORR4aKlX23WB3teaVMjXLuRKrD5GAMN8NSCR1vaetxBA==", env.Signatures[0].Sig)
}

func TestVerifyEnvelope(t *testing.T) {
	env, err := createSignedEnvelope()
	if err != nil {
		t.Fatal(err)
	}

	publicKeyPath := filepath.Join("test-data", "test-key.pub")
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	key, err := tuf.LoadKeyFromBytes(publicKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	verifier, err := signerverifier.NewSignerVerifierFromTUFKey(key)
	if err != nil {
		t.Fatal(err)
	}

	assert.Nil(t, VerifyEnvelope(context.Background(), env, []sslibdsse.Verifier{verifier}, 1))
}

func createSignedEnvelope() (*sslibdsse.Envelope, error) {
	privateKeyPath := filepath.Join("test-data", "test-key")
	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return nil, err
	}

	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(privateKeyBytes)
	if err != nil {
		return nil, err
	}

	message := []byte("test payload")
	payload := base64.StdEncoding.EncodeToString(message)

	env := &sslibdsse.Envelope{
		PayloadType: "application/vnd.gittuf+text",
		Payload:     payload,
		Signatures:  []sslibdsse.Signature{},
	}

	env, err = SignEnvelope(context.Background(), env, signer)
	if err != nil {
		return nil, err
	}

	return env, nil
}
