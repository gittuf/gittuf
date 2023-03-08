package signers

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"
)

type Ed25519SignerVerifier struct {
	keyID   string
	private ed25519.PrivateKey
	public  ed25519.PublicKey
}

func NewEd25519SignerVerifierFromSecureSystemsLibFormat(keyContents []byte) (*Ed25519SignerVerifier, error) {
	var k interface{}
	if err := json.Unmarshal(keyContents, &k); err != nil {
		return &Ed25519SignerVerifier{}, err
	}

	t := k.(map[string]interface{})
	keyID := t["keyid"].(string)
	keyval := t["keyval"].(map[string]interface{})

	publicS := keyval["public"].(string)
	public, err := hex.DecodeString(publicS)
	if err != nil {
		return &Ed25519SignerVerifier{}, err
	}

	if _, ok := keyval["private"]; ok {
		privateS := keyval["private"].(string)
		private, err := hex.DecodeString(privateS)
		if err != nil {
			return &Ed25519SignerVerifier{}, err
		}
		if len(private) == ed25519.PrivateKeySize/2 {
			private = append(private, public...)
		}
		return &Ed25519SignerVerifier{
			public: public,
			keyID:  keyID,
		}, nil
	}

	return &Ed25519SignerVerifier{
		public: public,
		keyID:  keyID,
	}, nil
}

func (sv *Ed25519SignerVerifier) Sign(ctx context.Context, data []byte) ([]byte, error) {
	if len(sv.private) == 0 {
		return []byte{}, ErrNotPrivateKey
	}

	signature := ed25519.Sign(sv.private, data) // Should we use a digest?
	return signature, nil
}

func (sv Ed25519SignerVerifier) Verify(ctx context.Context, data []byte, sig []byte) error {
	if ok := ed25519.Verify(sv.public, data, sig); ok { // Should we use a digest?
		return nil
	}
	return ErrSignatureVerificationFailed
}

func (sv Ed25519SignerVerifier) KeyID() (string, error) {
	return sv.keyID, nil
}

func (sv Ed25519SignerVerifier) Public() crypto.PublicKey {
	return sv.public
}
