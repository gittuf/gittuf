package gittuf

import (
	"crypto/ed25519"
	"encoding/json"
	"os"

	tufdata "github.com/theupdateframework/go-tuf/data"
)

func LoadEd25519PrivateKeyFromSslib(path string) (tufdata.PrivateKey, error) {
	var privKey tufdata.PrivateKey
	privKeyData, err := os.ReadFile(path)
	if err != nil {
		return tufdata.PrivateKey{}, err
	}
	err = json.Unmarshal(privKeyData, &privKey)
	if err != nil {
		return tufdata.PrivateKey{}, err
	}

	var keyValue KeyValue
	err = json.Unmarshal(privKey.Value, &keyValue)
	if err != nil {
		return tufdata.PrivateKey{}, err
	}
	/*
		Here, the assumption is that the key pair is in the securesystemslib
		format. However, the default python-sslib format does not contain the
		private and the public halves of the key in the "private" field as
		go-tuf expects. So, while a keypair can be generated using python-sslib,
		the public portion must be appended to the private portion in the JSON
		representation.
	*/
	if len(keyValue.Private) < ed25519.PrivateKeySize {
		fullPrivateValue, err := json.Marshal(KeyValue{
			Private: append(keyValue.Private, keyValue.Public...),
			Public:  keyValue.Public,
		})
		if err != nil {
			return tufdata.PrivateKey{}, err
		}
		return tufdata.PrivateKey{
			Type:       privKey.Type,
			Scheme:     privKey.Scheme,
			Algorithms: privKey.Algorithms,
			Value:      fullPrivateValue,
		}, nil
	}

	return privKey, nil
}

func GetEd25519PublicKeyFromPrivateKey(privKey *tufdata.PrivateKey) (tufdata.PublicKey, error) {
	var keyValue KeyValue

	err := json.Unmarshal(privKey.Value, &keyValue)
	if err != nil {
		return tufdata.PublicKey{}, err
	}

	newValue, err := json.Marshal(KeyValue{
		Private: []byte{},
		Public:  keyValue.Public,
	})
	if err != nil {
		return tufdata.PublicKey{}, err
	}

	return tufdata.PublicKey{
		Type:       privKey.Type,
		Scheme:     privKey.Scheme,
		Algorithms: privKey.Algorithms,
		Value:      newValue,
	}, nil

}

type KeyValue struct {
	Private []byte `json:"private,omitempty"`
	Public  []byte `json:"public,omitempty"`
}
