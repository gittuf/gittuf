// SPDX-License-Identifier: Apache-2.0

package common

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"github.com/spf13/cobra"
)

const (
	GPGKeyPrefix = "gpg:"
	FulcioPrefix = "fulcio:"
)

// LoadPublicKey returns a tuf.Key object for a PGP / Sigstore Fulcio / SSH
// (on-disk) key for use in gittuf metadata.
func LoadPublicKey(key string) (*tuf.Key, error) {
	var keyObj *tuf.Key

	switch {
	case strings.HasPrefix(key, GPGKeyPrefix):
		fingerprint := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(key, GPGKeyPrefix)))

		command := exec.Command("gpg", "--export", "--armor", fingerprint)
		stdOut, err := command.Output()
		if err != nil {
			return nil, err
		}

		keyObj, err = gpg.LoadGPGKeyFromBytes(stdOut)
		if err != nil {
			return nil, err
		}
	case strings.HasPrefix(key, FulcioPrefix):
		keyID := strings.TrimPrefix(key, FulcioPrefix)
		ks := strings.Split(keyID, "::")
		if len(ks) != 2 {
			return nil, fmt.Errorf("incorrect format for fulcio identity")
		}

		keyObj = &sslibsv.SSLibKey{
			KeyID:   keyID,
			KeyType: signerverifier.FulcioKeyType,
			Scheme:  signerverifier.FulcioKeyScheme,
			KeyVal: sslibsv.KeyVal{
				Identity: ks[0],
				Issuer:   ks[1],
			},
		}
	default:
		kb, err := os.ReadFile(key)
		if err != nil {
			return nil, err
		}

		keyObj, err = tuf.LoadKeyFromBytes(kb)
		if err != nil {
			return nil, err
		}
	}

	return keyObj, nil
}

// LoadSigner loads a signer for the specified key bytes. The key must be
// encoded either in a standard PEM format. For now, the custom securesystemslib
// format is also supported.
func LoadSigner(keyBytes []byte) (sslibdsse.SignerVerifier, error) {
	signer, err := sslibsv.NewSignerVerifierFromPEM(keyBytes)
	if err == nil {
		return signer, nil
	}

	return signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes) //nolint:staticcheck
}

func CheckIfSigningViable(_ *cobra.Command, _ []string) error {
	_, _, err := gitinterface.GetSigningCommand()

	return err
}
