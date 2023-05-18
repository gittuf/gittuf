package signerverifier

import (
	"encoding/json"

	"github.com/adityasaky/gittuf/internal/signerverifier/common"
	"github.com/adityasaky/gittuf/internal/signerverifier/ed25519"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
)

func NewSignerVerifierFromTUFKey(key *tuf.Key) (dsse.SignerVerifier, error) {
	switch key.KeyType {
	case ed25519.Ed25519KeyType:
		return ed25519.NewSignerVerifierFromTUFKey(key)
	}
	return nil, common.ErrUnknownKeyType
}

func NewSignerVerifierFromSecureSystemsLibFormat(keyContents []byte) (dsse.SignerVerifier, error) {
	k := key{}
	if err := json.Unmarshal(keyContents, &k); err != nil {
		// FIXME
		return nil, err
	}

	switch k.KeyType {
	case ed25519.Ed25519KeyType:
		return ed25519.NewSignerVerifierFromSecureSystemsLibFormat(keyContents)
	}

	return nil, common.ErrUnknownKeyType
}

// key is used only to load keys from file. Unlike tuf.Key, it includes the
// private portion of the key. In future, it'll be removed altogether in favour
// of go-securesystemslib's semantics.
type key struct {
	KeyType             string   `json:"keytype"`
	KeyID               string   `json:"keyid"`
	KeyVal              keyval   `json:"keyval"`
	Scheme              string   `json:"scheme"`
	KeyIDHashAlgorithms []string `json:"keyid_hash_algorithms"`
}

type keyval struct {
	Public  string `json:"public"`
	Private string `json:"private"`
}
