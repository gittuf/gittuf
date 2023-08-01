package common

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/adityasaky/gittuf/internal/signerverifier"
	sslibsv "github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

const (
	GPGKeyPrefix = "gpg:"
	FulcioPrefix = "fulcio:"
)

// ReadKeyBytes returns public key bytes using the custom securesystemslib
// format. It uses the underlying gpg binary to import a PGP key.
func ReadKeyBytes(key string) ([]byte, error) {
	var (
		kb  []byte
		err error
	)

	if strings.HasPrefix(key, GPGKeyPrefix) {
		fingerprint := strings.ToLower(strings.TrimSpace(strings.TrimPrefix(key, GPGKeyPrefix)))

		command := exec.Command("gpg", "--export", "--armor", fingerprint)
		stdOut, err := command.Output()
		if err != nil {
			return nil, err
		}

		publicKey := strings.TrimSpace(string(stdOut))
		pgpKey := &sslibsv.SSLibKey{
			KeyID:   fingerprint,
			KeyType: signerverifier.GPGKeyType,
			Scheme:  signerverifier.GPGKeyType, // TODO: this should use the underlying key algorithm
			KeyVal: sslibsv.KeyVal{
				Public: publicKey,
			},
		}

		kb, err = json.Marshal(pgpKey)
		if err != nil {
			return nil, err
		}
	} else if strings.HasPrefix(key, FulcioPrefix) {
		keyID := strings.TrimPrefix(key, FulcioPrefix)
		ks := strings.Split(keyID, "::")
		if len(ks) != 2 {
			return nil, fmt.Errorf("incorrect format for fulcio identity")
		}

		fulcioKey := &sslibsv.SSLibKey{
			KeyID:   keyID,
			KeyType: sslibsv.SigstoreKeyType,
			Scheme:  sslibsv.SigstoreKeyScheme,
			KeyVal: sslibsv.KeyVal{
				Identity: ks[0],
				Issuer:   ks[1],
			},
		}

		kb, err = json.Marshal(fulcioKey)
		if err != nil {
			return nil, err
		}
	} else {
		kb, err = os.ReadFile(key)
		if err != nil {
			return nil, err
		}
	}

	return kb, nil
}
