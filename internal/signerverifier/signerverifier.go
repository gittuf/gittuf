// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package signerverifier

import (
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
