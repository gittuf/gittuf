package signerverifier

import (
	"github.com/adityasaky/gittuf/internal/signerverifier/common"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/secure-systems-lab/go-securesystemslib/dsse"
	sslibsv "github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

func NewSignerVerifierFromTUFKey(key *tuf.Key) (dsse.SignerVerifier, error) {
	switch key.KeyType {
	case sslibsv.ED25519KeyType:
		return sslibsv.NewED25519SignerVerifierFromSSLibKey(key)
	case sslibsv.ECDSAKeyType:
		return sslibsv.NewECDSASignerVerifierFromSSLibKey(key)
	case sslibsv.RSAKeyType:
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
