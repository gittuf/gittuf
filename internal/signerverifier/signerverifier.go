// SPDX-License-Identifier: Apache-2.0

package signerverifier

import (
	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	sslibsv "github.com/secure-systems-lab/go-securesystemslib/signerverifier"
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

func NewSignerVerifierFromTUFKey(key *tuf.Key) (dsse.SignerVerifier, error) {
	switch key.KeyType {
	case ED25519KeyType:
		return sslibsv.NewED25519SignerVerifierFromSSLibKey(key)
	case ECDSAKeyType:
		return sslibsv.NewECDSASignerVerifierFromSSLibKey(key)
	case RSAKeyType:
		return sslibsv.NewRSAPSSSignerVerifierFromSSLibKey(key)
	}
	return nil, common.ErrUnknownKeyType
}

func NewSignerVerifierFromSecureSystemsLibFormat(keyContents []byte) (dsse.SignerVerifier, error) {
	key, err := tuf.LoadKeyFromBytes(keyContents)
	if err != nil {
		return nil, err
	}

	return NewSignerVerifierFromTUFKey(key)
}
