package gittuf

import (
	"encoding/json"
	"log"
	"os/exec"
	"time"

	tuf "github.com/theupdateframework/go-tuf/data"
	tufkeys "github.com/theupdateframework/go-tuf/pkg/keys"
	tufsign "github.com/theupdateframework/go-tuf/sign"
)

func Init(rootKey tuf.PrivateKey, expires time.Time, keys ...tuf.PublicKey) error {
	cmd := exec.Command("git", "init")

	err := cmd.Run()
	if err != nil {
		log.Fatal(err)
	}

	rootRole := tuf.NewRoot()
	if !expires.IsZero() {
		rootRole.Expires = expires
	}
	for _, k := range keys {
		rootRole.AddKey(&k)
	}

	rootRoleJson, err := json.Marshal(rootRole)
	if err != nil {
		log.Fatal(err)
	}

	rootRoleToSign := tuf.Signed{
		Signed:     rootRoleJson,
		Signatures: []tuf.Signature{},
	}

	signer, err := tufkeys.GetSigner(&rootKey)
	if err != nil {
		log.Fatal(err)
	}
	tufsign.Sign(&rootRoleToSign, signer)

	// Write to .git

	return nil
}
