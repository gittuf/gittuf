package gittuf

import (
	"encoding/json"
	"os/exec"
	"time"

	tufdata "github.com/theupdateframework/go-tuf/data"
	tufkeys "github.com/theupdateframework/go-tuf/pkg/keys"
	tufsign "github.com/theupdateframework/go-tuf/sign"
)

func Init(rootKey tufdata.PrivateKey, expires time.Time, keys []tufdata.PublicKey) (tufdata.Signed, error) {
	cmd := exec.Command("git", "init")

	err := cmd.Run()
	if err != nil {
		return tufdata.Signed{}, err
	}

	rootRole := tufdata.NewRoot()
	if !expires.IsZero() {
		rootRole.Expires = expires
	}
	for _, k := range keys {
		rootRole.AddKey(&k)
	}

	rootRoleJson, err := json.Marshal(rootRole)
	if err != nil {
		return tufdata.Signed{}, err
	}

	rootRoleMb := tufdata.Signed{
		Signed:     rootRoleJson,
		Signatures: []tufdata.Signature{},
	}

	signer, err := tufkeys.GetSigner(&rootKey)
	if err != nil {
		return tufdata.Signed{}, err
	}
	tufsign.Sign(&rootRoleMb, signer)

	return rootRoleMb, nil
}
