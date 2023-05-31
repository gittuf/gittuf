package ed25519

import (
	"context"
	"crypto"
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"

	"github.com/adityasaky/gittuf/internal/signerverifier/common"
	"github.com/adityasaky/gittuf/internal/tuf"
)

const (
	Ed25519KeyType   = "ed25519"
	Ed25519KeyScheme = "ed25519"
)

type Ed25519SignerVerifier struct {
	keyID   string
	private ed25519.PrivateKey
	public  ed25519.PublicKey
}

// NewSignerVerifierFromSecureSystemsLibFormat creates an Ed25519SignerVerifier
// from a key serialized in the custom securesystemslib format. This API may be
// used when signing or verifying signatures, and so must handle private keys.
func NewSignerVerifierFromSecureSystemsLibFormat(keyContents []byte) (*Ed25519SignerVerifier, error) {
	type keyval struct {
		Public  string `json:"public"`
		Private string `json:"private"`
	}
	k := struct {
		KeyID               string   `json:"keyid"`
		KeyVal              keyval   `json:"keyval"`
		Scheme              string   `json:"scheme"`
		KeyIDHashAlgorithms []string `json:"keyid_hash_algorithms"`
	}{}

	if err := json.Unmarshal(keyContents, &k); err != nil {
		return nil, err
	}

	if len(k.KeyID) == 0 {
		key, err := tuf.NewKey(Ed25519KeyType, Ed25519KeyScheme, tuf.KeyVal{Public: k.KeyVal.Public}, k.KeyIDHashAlgorithms)
		if err != nil {
			return nil, err
		}
		keyID, err := key.ID()
		if err != nil {
			return nil, err
		}
		k.KeyID = keyID
	}

	public, err := hex.DecodeString(k.KeyVal.Public)
	if err != nil {
		return nil, err
	}

	if len(k.KeyVal.Private) > 0 {
		private, err := hex.DecodeString(k.KeyVal.Private)
		if err != nil {
			return nil, err
		}

		// python-securesystemslib provides an interface to generate ed25519
		// keys but it differs slightly in how it serializes the key to disk.
		// Specifically, the keyval.private field includes _only_ the private
		// portion of the key while libraries such as crypto/ed25519 also expect
		// the public portion. So, if the private portion is half of what we
		// expect, we append the public portion as well.
		if len(private) == ed25519.PrivateKeySize/2 {
			private = append(private, public...)
		}

		return &Ed25519SignerVerifier{
			private: private,
			public:  public,
			keyID:   k.KeyID,
		}, nil
	}

	return &Ed25519SignerVerifier{
		private: nil,
		public:  public,
		keyID:   k.KeyID,
	}, nil
}

// NewSignerVerifierFromTUFKey creates an Ed25519SignerVerifier from a tuf.Key
// instance. This is used when verifying a signature using a key in the
// metadata, and so we know it's not used for signing.
func NewSignerVerifierFromTUFKey(key *tuf.Key) (*Ed25519SignerVerifier, error) {
	kb, err := hex.DecodeString(key.KeyVal.Public)
	if err != nil {
		return nil, err
	}

	keyID, err := key.ID()
	if err != nil {
		return nil, err
	}

	return &Ed25519SignerVerifier{
		keyID:   keyID,
		public:  ed25519.PublicKey(kb),
		private: nil,
	}, nil
}

// Sign creates a signature for `data`.
func (sv *Ed25519SignerVerifier) Sign(ctx context.Context, data []byte) ([]byte, error) {
	if len(sv.private) == 0 {
		return nil, common.ErrNotPrivateKey
	}

	signature := ed25519.Sign(sv.private, data) // Should we use a digest?
	return signature, nil
}

// Verify verifies the `sig` value passed in against `data`.
func (sv Ed25519SignerVerifier) Verify(ctx context.Context, data []byte, sig []byte) error {
	if ok := ed25519.Verify(sv.public, data, sig); ok { // Should we use a digest?
		return nil
	}
	return common.ErrSignatureVerificationFailed
}

// KeyID returns the identifier of the key used to create the
// Ed25519SignerVerifier instance.
func (sv Ed25519SignerVerifier) KeyID() (string, error) {
	return sv.keyID, nil
}

// Public returns the public portion of the key used to create the
// Ed25519SignerVerifier instance.
func (sv Ed25519SignerVerifier) Public() crypto.PublicKey {
	return sv.public
}
