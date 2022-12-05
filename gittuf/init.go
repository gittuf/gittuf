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
	rootKeys []tufdata.PrivateKey,
	rootExpires time.Time,
	rootThreshold int,
	rootPubKeys []tufdata.PublicKey,
	targetsPubKeys []tufdata.PublicKey,
	targetsPrivKeys []tufdata.PrivateKey,
	targetsExpires time.Time,
	targetsThreshold int,
	initArgs ...string) (map[string]tufdata.Signed, error) {
	roles := map[string]tufdata.Signed{}

	args := []string{"init"}
	args = append(args, initArgs...)
	cmd := exec.Command("git", args...)
	err := cmd.Run()
	if err != nil {
		return roles, err
	}

	rootRole, err := initRoot(rootKeys, rootExpires, rootThreshold, rootPubKeys,
		targetsPubKeys, targetsThreshold)
	if err != nil {
		return roles, err
	}
	roles["root"] = rootRole

	targetsRole, err := initTargets(targetsPrivKeys, targetsExpires,
		targetsThreshold)
	if err != nil {
		return roles, err
	}
	roles["targets"] = targetsRole

	return roles, nil
}

func initRoot(
	keys []tufdata.PrivateKey,
	expires time.Time,
	rootThreshold int,
	rootPubKeys []tufdata.PublicKey,
	targetsPubKeys []tufdata.PublicKey,
	targetsThreshold int) (tufdata.Signed, error) {
	rootRole := tufdata.NewRoot()

	if !expires.IsZero() {
		rootRole.Expires = expires
	}

	rootRole.Version = 1

	pubKeys := append(rootPubKeys, targetsPubKeys...)

	for _, k := range pubKeys {
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

	var rootKeyIds []string
	for _, k := range rootPubKeys {
		rootKeyIds = append(rootKeyIds, k.IDs()...)
	}
	rootRoleMeta := tufdata.Role{
		KeyIDs:    rootKeyIds,
		Threshold: rootThreshold,
	}
	rootRole.Roles["root"] = &rootRoleMeta

	var targetsKeyIds []string
	for _, k := range targetsPubKeys {
		targetsKeyIds = append(targetsKeyIds, k.IDs()...)
	}
	targetsRoleMeta := tufdata.Role{
		KeyIDs:    targetsKeyIds,
		Threshold: targetsThreshold,
	}
	rootRole.Roles["targets"] = &targetsRoleMeta

	rootRoleJson, err := json.Marshal(rootRole)
	if err != nil {
		return tufdata.Signed{}, err
	}

	rootRoleMb := tufdata.Signed{
		Signed:     rootRoleJson,
		Signatures: []tufdata.Signature{},
	}

	for _, key := range keys {
		signer, err := tufkeys.GetSigner(&key)
		if err != nil {
			return tufdata.Signed{}, err
		}
		err = tufsign.Sign(&rootRoleMb, signer)
		if err != nil {
			return tufdata.Signed{}, err
		}
	}

	return rootRoleMb, nil
}

func initTargets(
	keys []tufdata.PrivateKey,
	expires time.Time,
	threshold int) (tufdata.Signed, error) {
	targetsRole := tufdata.NewTargets()

	if !expires.IsZero() {
		targetsRole.Expires = expires
	}

	targetsRole.Version = 1

	targetsRole.Delegations = &tufdata.Delegations{
		Keys:  map[string]*tufdata.PublicKey{},
		Roles: []tufdata.DelegatedRole{createAllowRule()},
	}

	targetsRoleJson, err := json.Marshal(targetsRole)
	if err != nil {
		return tufdata.Signed{}, err
	}

	targetsRoleMb := tufdata.Signed{
		Signed:     targetsRoleJson,
		Signatures: []tufdata.Signature{},
	}

	for _, key := range keys {
		signer, err := tufkeys.GetSigner(&key)
		if err != nil {
			return tufdata.Signed{}, err
		}
		err = tufsign.Sign(&targetsRoleMb, signer)
		if err != nil {
			return tufdata.Signed{}, err
		}
	}

	return targetsRoleMb, nil
}
