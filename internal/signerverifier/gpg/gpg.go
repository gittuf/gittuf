// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gpg

import (
	"bytes"
	"fmt"
	"strings"

	"github.com/ProtonMail/go-crypto/openpgp"
	"github.com/gittuf/gittuf/internal/signerverifier"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
)

// LoadGPGKeyFromBytes returns a tuf.Key for a GPG / PGP key passed in as
// armored bytes. The returned tuf.Key uses the primary key's fingerprint as the
// key ID.
func LoadGPGKeyFromBytes(contents []byte) (*tuf.Key, error) {
	keyring, err := openpgp.ReadArmoredKeyRing(bytes.NewReader(contents))
	if err != nil {
		return nil, err
	}

	// TODO: check if this is correct for subkeys
	fingerprint := fmt.Sprintf("%x", keyring[0].PrimaryKey.Fingerprint)
	publicKey := strings.TrimSpace(string(contents))

	gpgKey := &tuf.Key{
		KeyID:   fingerprint,
		KeyType: signerverifier.GPGKeyType,
		Scheme:  signerverifier.GPGKeyType, // TODO: this should use the underlying key algorithm
		KeyVal: sslibsv.KeyVal{
			Public: publicKey,
		},
	}

	return gpgKey, nil
}
