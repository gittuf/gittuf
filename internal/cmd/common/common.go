package common

import (
	"encoding/json"
	"os"
	"os/exec"
	"strings"

	"github.com/adityasaky/gittuf/internal/tuf"
)

func ReadKeyBytes(key string) ([]byte, error) {
	var (
		kb  []byte
		err error
	)
	if strings.HasPrefix(key, "gpg:") {
		fingerprint := strings.TrimSpace(strings.TrimPrefix(key, "gpg:"))

		command := exec.Command("gpg", "--export", "--armor", fingerprint)
		stdOut, err := command.Output()
		if err != nil {
			return nil, err
		}

		publicKey := strings.TrimSpace(string(stdOut))
		pgpKey := tuf.Key{
			KeyType: "gpg",
			Scheme:  "gpg", // TODO: this should use the underlying key algorithm
			KeyVal: tuf.KeyVal{
				Public: publicKey,
			},
		}

		kb, err = json.Marshal(pgpKey)
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
