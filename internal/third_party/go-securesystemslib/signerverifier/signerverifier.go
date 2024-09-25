package signerverifier

import (
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/x509"
	"encoding/hex"
	"errors"
	"strings"

	"github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
)

var KeyIDHashAlgorithms = []string{"sha256", "sha512"}

var (
	ErrPrivateKey                  = errors.New("key must be a public key")
	ErrNotPrivateKey               = errors.New("loaded key is not a private key")
	ErrSignatureVerificationFailed = errors.New("failed to verify signature")
	ErrUnknownKeyType              = errors.New("unknown key type")
	ErrInvalidThreshold            = errors.New("threshold is either less than 1 or greater than number of provided public keys")
	ErrInvalidKey                  = errors.New("key object has no value")
	ErrInvalidPEM                  = errors.New("unable to parse PEM block")
)

const (
	PublicKeyPEM  = "PUBLIC KEY"
	PrivateKeyPEM = "PRIVATE KEY"
)

type SSLibKey struct {
	KeyIDHashAlgorithms []string `json:"keyid_hash_algorithms"`
	KeyType             string   `json:"keytype"`
	KeyVal              KeyVal   `json:"keyval"`
	Scheme              string   `json:"scheme"`
	KeyID               string   `json:"keyid"`
}

type KeyVal struct {
	Public      string `json:"public,omitempty"`
	Certificate string `json:"certificate,omitempty"`
	Identity    string `json:"identity,omitempty"`
	Issuer      string `json:"issuer,omitempty"`
}

// LoadKey returns an SSLibKey object when provided a PEM encoded key.
// Currently, RSA, ED25519, and ECDSA keys are supported.
func LoadKey(keyBytes []byte) (*SSLibKey, error) {
	_, rawKey, err := decodeAndParsePEM(keyBytes)
	if err != nil {
		return nil, err
	}

	return NewKey(rawKey)
}

// NewKey returns an SSLibKey object for an RSA, ECDSA, or ED25519 public key.
func NewKey(rawKey any) (*SSLibKey, error) {
	var key *SSLibKey
	switch k := rawKey.(type) {
	case *rsa.PublicKey:
		pubKeyBytes, err := x509.MarshalPKIXPublicKey(k)
		if err != nil {
			return nil, err
		}
		key = &SSLibKey{
			KeyIDHashAlgorithms: KeyIDHashAlgorithms,
			KeyType:             RSAKeyType,
			KeyVal: KeyVal{
				Public: strings.TrimSpace(string(generatePEMBlock(pubKeyBytes, PublicKeyPEM))),
			},
			Scheme: RSAKeyScheme,
		}

	case ed25519.PublicKey:
		key = &SSLibKey{
			KeyIDHashAlgorithms: KeyIDHashAlgorithms,
			KeyType:             ED25519KeyType,
			KeyVal: KeyVal{
				Public: strings.TrimSpace(hex.EncodeToString(k)),
			},
			Scheme: ED25519KeyType,
		}

	case *ecdsa.PublicKey:
		pubKeyBytes, err := x509.MarshalPKIXPublicKey(k)
		if err != nil {
			return nil, err
		}
		key = &SSLibKey{
			KeyIDHashAlgorithms: KeyIDHashAlgorithms,
			KeyType:             ECDSAKeyType,
			KeyVal: KeyVal{
				Public: strings.TrimSpace(string(generatePEMBlock(pubKeyBytes, PublicKeyPEM))),
			},
			Scheme: ECDSAKeyScheme,
		}

	case *rsa.PrivateKey, ed25519.PrivateKey, *ecdsa.PrivateKey:
		return nil, ErrPrivateKey

	default:
		return nil, ErrUnknownKeyType
	}

	keyID, err := calculateKeyID(key)
	if err != nil {
		return nil, err
	}
	key.KeyID = keyID

	return key, nil
}

func NewSignerVerifierFromPEM(keyBytes []byte) (dsse.SignerVerifier, error) {
	_, rawKey, err := decodeAndParsePEM(keyBytes)
	if err != nil {
		return nil, err
	}

	switch k := rawKey.(type) {
	case *rsa.PrivateKey:
		publicKey := k.Public()
		sslibKey, err := NewKey(publicKey)
		if err != nil {
			return nil, err
		}
		return &RSAPSSSignerVerifier{
			private: k,
			public:  publicKey.(*rsa.PublicKey),
			keyID:   sslibKey.KeyID,
		}, nil

	case *rsa.PublicKey:
		sslibKey, err := NewKey(k)
		if err != nil {
			return nil, err
		}
		return &RSAPSSSignerVerifier{
			public: k,
			keyID:  sslibKey.KeyID,
		}, nil

	case ed25519.PrivateKey:
		publicKey := k.Public()
		sslibKey, err := NewKey(publicKey)
		if err != nil {
			return nil, err
		}
		return &ED25519SignerVerifier{
			PrivateKey: k,
			PublicKey:  publicKey.(ed25519.PublicKey),
			ID:         sslibKey.KeyID,
		}, nil

	case ed25519.PublicKey:
		sslibKey, err := NewKey(k)
		if err != nil {
			return nil, err
		}
		return &ED25519SignerVerifier{
			PublicKey: k,
			ID:        sslibKey.KeyID,
		}, nil

	case *ecdsa.PrivateKey:
		publicKey := k.Public()
		sslibKey, err := NewKey(publicKey)
		if err != nil {
			return nil, err
		}
		return &ECDSASignerVerifier{
			private: k,
			public:  publicKey.(*ecdsa.PublicKey),
			keyID:   sslibKey.KeyID,
		}, nil

	case *ecdsa.PublicKey:
		sslibKey, err := NewKey(k)
		if err != nil {
			return nil, err
		}
		return &ECDSASignerVerifier{
			public: k,
			keyID:  sslibKey.KeyID,
		}, nil
	}

	return nil, ErrUnknownKeyType
}

func NewVerifierFromSSLibKey(key *SSLibKey) (dsse.SignerVerifier, error) {
	switch key.KeyType {
	case RSAKeyType:
		return NewRSAPSSSignerVerifierFromSSLibKey(key)
	case ED25519KeyType:
		return NewED25519SignerVerifierFromSSLibKey(key)
	case ECDSAKeyType:
		return NewECDSASignerVerifierFromSSLibKey(key)
	default:
		return nil, ErrUnknownKeyType
	}
}
