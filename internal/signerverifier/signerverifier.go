// SPDX-License-Identifier: Apache-2.0

package signerverifier

import (
	"crypto/ed25519"
	"encoding/hex"
	"encoding/json"

	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

const (
	ED25519KeyType  = sslibsv.ED25519KeyType
	ECDSAKeyType    = sslibsv.ECDSAKeyType
	RSAKeyType      = sslibsv.RSAKeyType
	GPGKeyType      = "gpg"
	FulcioKeyType   = "sigstore-oidc"
	FulcioKeyScheme = "fulcio"
	RekorServer     = "https://rekor.sigstore.dev"
)

type legacyPrivateKey struct {
	KeyIDHashAlgorithms []string `json:"keyid_hash_algorithms"`
	KeyType             string   `json:"keytype"`
	KeyVal              keyVal   `json:"keyval"`
	Scheme              string   `json:"scheme"`
	KeyID               string   `json:"keyid"`
}

type keyVal struct {
	Private string `json:"private"`
	Public  string `json:"public"`
}

// NewSignerVerifierFromTUFKey returns a verifier for RSA, ED25519, and ECDSA
// keys. While this is called signerverifier, tuf.Key only supports public keys.
//
// Deprecated: Switch to upstream key loading APIs.
func NewSignerVerifierFromTUFKey(key *tuf.Key) (dsse.Verifier, error) {
	switch key.KeyType {
	case ED25519KeyType:
		return sslibsv.NewED25519SignerVerifierFromSSLibKey(key)
	case ECDSAKeyType:
		return sslibsv.NewECDSASignerVerifierFromSSLibKey(key)
	case RSAKeyType:
		return sslibsv.NewRSAPSSSignerVerifierFromSSLibKey(key)
	case ssh.SSHKeyType:
		return ssh.NewVerifierFromKey(key)
	}
	return nil, common.ErrUnknownKeyType
}

// NewSignerVerifierFromSecureSystemsLibFormat parses the bytes of a public or
// private key in the legacy sslib encoding format. This will eventually be
// removed as gittuf switches to standard on-disk key serialization.
//
// Deprecated: This has been added temporarily for support with old on-disk key
// formats.
func NewSignerVerifierFromSecureSystemsLibFormat(keyContents []byte) (dsse.SignerVerifier, error) {
	key := &legacyPrivateKey{}
	if err := json.Unmarshal(keyContents, key); err != nil {
		return nil, err
	}

	switch key.KeyType {
	case RSAKeyType:
		if key.KeyVal.Private != "" {
			return sslibsv.NewSignerVerifierFromPEM([]byte(key.KeyVal.Private))
		}
		return sslibsv.NewSignerVerifierFromPEM([]byte(key.KeyVal.Public))
	case ECDSAKeyType:
		if key.KeyVal.Private != "" {
			return sslibsv.NewSignerVerifierFromPEM([]byte(key.KeyVal.Private))
		}
		return sslibsv.NewSignerVerifierFromPEM([]byte(key.KeyVal.Public))
	case ED25519KeyType:
		publicBytes, err := hex.DecodeString(key.KeyVal.Public)
		if err != nil {
			return nil, err
		}
		if key.KeyVal.Private != "" {
			privateBytes, err := hex.DecodeString(key.KeyVal.Private)
			if err != nil {
				return nil, err
			}
			if len(privateBytes) == ed25519.PrivateKeySize/2 {
				// compatibility with old py-sslib generated keys, where the
				// private key didn't include the public bytes
				privateBytes = append(privateBytes, publicBytes...)
			}
			return &sslibsv.ED25519SignerVerifier{
				PrivateKey: ed25519.PrivateKey(privateBytes),
				PublicKey:  ed25519.PublicKey(publicBytes),
				ID:         key.KeyID,
			}, nil
		}
		return &sslibsv.ED25519SignerVerifier{
			PublicKey: ed25519.PublicKey(publicBytes),
			ID:        key.KeyID,
		}, nil
	default:
		return nil, sslibsv.ErrUnknownKeyType
	}
}
