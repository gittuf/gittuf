// SPDX-License-Identifier: Apache-2.0

package common

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibsv "github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/spf13/cobra"
)

const (
	GPGKeyPrefix = "gpg:"
	FulcioPrefix = "fulcio:"
	EvalModeKey  = "GITTUF_EVAL"
)

var ErrNotInEvalMode = fmt.Errorf("this feature is only available with eval mode, and can UNDERMINE repository security; override by setting %s=1", EvalModeKey)

// ReadKeyBytes returns public key bytes using the custom securesystemslib
// format. It uses the underlying gpg binary to import a PGP key.
//
// Deprecated: With key format standardization, we can use tuf.Key for public
// keys. Use LoadKey to get a tuf.Key object directly. This deprecation can only
// be complete with API changes in repository to handle a signer object for the
// private key bits. Then, we'll have retired passing around key bytes
// altogether.
func ReadKeyBytes(key string) ([]byte, error) {
	var (
		kb  []byte
		err error
	)

	switch {
	case strings.HasPrefix(key, GPGKeyPrefix):
		fingerprint := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(key, GPGKeyPrefix)))

		command := exec.Command("gpg", "--export", "--armor", fingerprint)
		stdOut, err := command.Output()
		if err != nil {
			return nil, err
		}

		pgpKey, err := gpg.LoadGPGKeyFromBytes(stdOut)
		if err != nil {
			return nil, err
		}

		kb, err = json.Marshal(pgpKey)
		if err != nil {
			return nil, err
		}
	case strings.HasPrefix(key, FulcioPrefix):
		keyID := strings.TrimPrefix(key, FulcioPrefix)
		ks := strings.Split(keyID, "::")
		if len(ks) != 2 {
			return nil, fmt.Errorf("incorrect format for fulcio identity")
		}

		fulcioKey := &sslibsv.SSLibKey{
			KeyID:   keyID,
			KeyType: signerverifier.FulcioKeyType,
			Scheme:  signerverifier.FulcioKeyScheme,
			KeyVal: sslibsv.KeyVal{
				Identity: ks[0],
				Issuer:   ks[1],
			},
		}

		kb, err = json.Marshal(fulcioKey)
		if err != nil {
			return nil, err
		}
	default:
		kb, err = os.ReadFile(key)
		if err != nil {
			return nil, err
		}

		// With PEM support, we can't assume this is right. For support in the
		// interim, we load the key and serialize it again.
		keyObj, err := tuf.LoadKeyFromBytes(kb)
		if err != nil {
			return nil, err
		}

		kb, err = json.Marshal(keyObj)
		if err != nil {
			return nil, err
		}
	}

	return kb, nil
}

// LoadKey returns a tuf.Key object for a PGP / Sigstore Fulcio / SSH (on-disk)
// key for use in gittuf metadata.
func LoadKey(key string) (*tuf.Key, error) {
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

func CheckIfSigningViable(_ *cobra.Command, _ []string) error {
	_, _, err := gitinterface.GetSigningCommand()

	return err
}

func EvalMode() bool {
	return os.Getenv(EvalModeKey) == "1"
}
