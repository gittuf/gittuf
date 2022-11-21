package gittuf

import (
	"crypto/ed25519"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	tufdata "github.com/theupdateframework/go-tuf/data"
	tufkeys "github.com/theupdateframework/go-tuf/pkg/keys"
	tufsign "github.com/theupdateframework/go-tuf/sign"
	tufverify "github.com/theupdateframework/go-tuf/verify"
)

var METADATADIR = "../metadata" // FIXME: embed metadata in Git repo

// FIXME: update load... methods to be generic of type

func loadRootRole(roleName string) (tufdata.Root, error) {
	var role tufdata.Root
	// TODO: Pass in pubkey and verify signature
	// TODO: Update to handle metadata embedded in repo
	roleJson, err := os.ReadFile(filepath.Join(METADATADIR, fmt.Sprintf("%s.json", roleName)))
	if err != nil {
		return role, err
	}

	var roleMb tufdata.Signed
	err = json.Unmarshal(roleJson, &roleMb)
	if err != nil {
		return role, err
	}

	err = json.Unmarshal(roleMb.Signed, &role)
	if err != nil {
		return role, err
	}

	return role, nil
}

func loadTargetsRole(roleName string, db *tufverify.DB) (tufdata.Targets, error) {
	var role tufdata.Targets
	// TODO: Update to handle metadata embedded in repo
	roleJson, err := os.ReadFile(filepath.Join(METADATADIR, fmt.Sprintf("%s.json", roleName)))
	if err != nil {
		return role, err
	}

	var roleMb tufdata.Signed
	err = json.Unmarshal(roleJson, &roleMb)
	if err != nil {
		return role, err
	}

	// FIXME: Activate sig verification
	// err = db.VerifySignatures(&roleMb, roleName)
	// if err != nil {
	// 	return role, err
	// }

	err = json.Unmarshal(roleMb.Signed, &role)
	if err != nil {
		return role, err
	}

	return role, nil
}

func LoadEd25519PublicKeyFromSslib(path string) (tufdata.PublicKey, error) {
	var pubKey tufdata.PublicKey
	pubKeyData, err := os.ReadFile(path)
	if err != nil {
		return tufdata.PublicKey{}, err
	}
	err = json.Unmarshal(pubKeyData, &pubKey)
	if err != nil {
		return tufdata.PublicKey{}, err
	}

	return pubKey, nil
}

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

func generateAndSignMbFromStruct(content interface{}, keys []tufdata.PrivateKey) (tufdata.Signed, error) {
	var newMb tufdata.Signed
	newJson, err := json.Marshal(content)
	if err != nil {
		return newMb, err
	}
	newMb = tufdata.Signed{
		Signed:     newJson,
		Signatures: []tufdata.Signature{},
	}
	for _, key := range keys {
		signer, err := tufkeys.GetSigner(&key)
		if err != nil {
			return newMb, err
		}
		err = tufsign.Sign(&newMb, signer)
		if err != nil {
			return newMb, err
		}
	}
	return newMb, nil
}
