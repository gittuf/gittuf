package gittuf

import (
	"encoding/json"
	"os/exec"
	"time"

	tufdata "github.com/theupdateframework/go-tuf/data"
	tufkeys "github.com/theupdateframework/go-tuf/pkg/keys"
	tufsign "github.com/theupdateframework/go-tuf/sign"
)

func Init(
	rootKey tufdata.PrivateKey,
	rootExpires time.Time,
	keys []tufdata.PublicKey,
	targetsKey tufdata.PrivateKey,
	targetsExpires time.Time) (map[string]tufdata.Signed, error) {
	roles := map[string]tufdata.Signed{}

	cmd := exec.Command("git", "init")
	err := cmd.Run()
	if err != nil {
		return roles, err
	}

	rootRole, err := initRoot(rootKey, rootExpires, keys)
	if err != nil {
		return roles, err
	}
	roles["root"] = rootRole

	targetsRole, err := initTargets(targetsKey, targetsExpires)
	if err != nil {
		return roles, err
	}
	roles["targets"] = targetsRole

	return roles, nil
}

func initRoot(key tufdata.PrivateKey, expires time.Time, keys []tufdata.PublicKey) (tufdata.Signed, error) {
	rootRole := tufdata.NewRoot()

	if !expires.IsZero() {
		rootRole.Expires = expires
	}

	rootRole.Version = 1

	for _, k := range keys {
		/*
			 FIXME:
				tufdata.Root.Keys is of type map[string]*tufdata.PublicKey
				and passing &k in directly will result in the last public key in
				the slice being associated with each entry.
		*/
		tmpKey := tufdata.PublicKey{
			Type:       k.Type,
			Algorithms: k.Algorithms,
			Value:      k.Value,
			Scheme:     k.Scheme,
		}
		rootRole.AddKey(&tmpKey)
	}

	rootRoleJson, err := json.Marshal(rootRole)
	if err != nil {
		return tufdata.Signed{}, err
	}

	rootRoleMb := tufdata.Signed{
		Signed:     rootRoleJson,
		Signatures: []tufdata.Signature{},
	}

	signer, err := tufkeys.GetSigner(&key)
	if err != nil {
		return tufdata.Signed{}, err
	}
	tufsign.Sign(&rootRoleMb, signer)

	return rootRoleMb, nil
}

func initTargets(key tufdata.PrivateKey, expires time.Time) (tufdata.Signed, error) {
	targetsRole := tufdata.NewTargets()

	if !expires.IsZero() {
		targetsRole.Expires = expires
	}

	targetsRole.Version = 1

	targetsRoleJson, err := json.Marshal(targetsRole)
	if err != nil {
		return tufdata.Signed{}, err
	}

	targetsRoleMb := tufdata.Signed{
		Signed:     targetsRoleJson,
		Signatures: []tufdata.Signature{},
	}

	signer, err := tufkeys.GetSigner(&key)
	if err != nil {
		return tufdata.Signed{}, err
	}
	tufsign.Sign(&targetsRoleMb, signer)

	return targetsRoleMb, nil
}
