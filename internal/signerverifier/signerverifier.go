// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package signerverifier

import (
	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

const (
	GPGKeyType      = "gpg"
	FulcioKeyType   = "sigstore-oidc"
	FulcioKeyScheme = "fulcio"
)

// NewSignerVerifierFromTUFKey returns a verifier for RSA, ED25519, and ECDSA
// keys. While this is called signerverifier, tuf.Key only supports public keys.
//
// Deprecated: Switch to upstream key loading APIs.
func NewSignerVerifierFromTUFKey(key *tuf.Key) (dsse.Verifier, error) {
	switch key.KeyType { //nolint:gocritic
	case ssh.SSHKeyType:
		return ssh.NewVerifierFromKey(key)
	}
	return nil, common.ErrUnknownKeyType
}
