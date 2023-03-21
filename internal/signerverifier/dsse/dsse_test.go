package dsse

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	d "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

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

	assert.Nil(t, VerifyEnvelope(context.Background(), env, [][]byte{publicKeyBytes}, 1))
}

func createSignedEnvelope() (*d.Envelope, error) {
	privateKeyPath := filepath.Join("test-data", "test-key")
	privateKeyBytes, err := os.ReadFile(privateKeyPath)
	if err != nil {
		return &d.Envelope{}, err
	}

	message := []byte("test payload")
	payload := base64.StdEncoding.EncodeToString(message)

	env := &d.Envelope{
		PayloadType: "application/vnd.gittuf+text",
		Payload:     payload,
		Signatures:  []d.Signature{},
	}

	env, err = SignEnvelope(context.Background(), env, privateKeyBytes)
	if err != nil {
		return &d.Envelope{}, err
	}

	return env, nil
}
