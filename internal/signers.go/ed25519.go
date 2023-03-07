package signers

import (
	"context"
	"crypto/ed25519"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
)

// Horrific

var ErrSignatureVerificationFailed = errors.New("failed to verify signature")

type Ed25519SignerVerifier struct {
	keyID   string
	private ed25519.PrivateKey
	public  ed25519.PublicKey
}

func NewEd25519SignerFromSslibKey(keyContents []byte) (*Ed25519SignerVerifier, error) {
	var k interface{}
	if err := json.Unmarshal(keyContents, &k); err != nil {
		return &Ed25519SignerVerifier{}, err
	}

	t := k.(map[string]interface{})
	keyval := t["keyval"].(map[string]interface{})

	privateS := keyval["private"].(string)
	private, err := hex.DecodeString(privateS)
	if err != nil {
		return &Ed25519SignerVerifier{}, err
	}

	publicS := keyval["public"].(string)
	public, err := hex.DecodeString(publicS)
	if err != nil {
		return &Ed25519SignerVerifier{}, err
	}

	return &Ed25519SignerVerifier{
		private: private,
		public:  public,
	}, nil
}

func (sv *Ed25519SignerVerifier) Sign(ctx context.Context, data []byte) ([]byte, error) {
	digest := sha256.Sum256(data)
	signature := ed25519.Sign(sv.private, digest[:])
	return signature, nil
}

func (sv *Ed25519SignerVerifier) Verify(ctx context.Context, data []byte, sig []byte) error {
	if ok := ed25519.Verify(sv.public, data, sig); ok {
		return nil
	}
	return ErrSignatureVerificationFailed
}

func (sv *Ed25519SignerVerifier) KeyID() (string, error) {
	return sv.keyID, nil
}

func (sv *Ed25519SignerVerifier) Public() []byte {
	return sv.public
}
