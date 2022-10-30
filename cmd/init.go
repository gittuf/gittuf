package cmd

import (
	"encoding/json"
	"os"

	"github.com/adityasaky/gittuf/gittuf"
	"github.com/spf13/cobra"
	tufdata "github.com/theupdateframework/go-tuf/data"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a Git repository with TUF support",
	RunE:  runInit,
}

var (
	privKeyPath    string
	expires        string
	publicKeyPaths []string
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(
		&privKeyPath,
		"private-key",
		"k",
		"",
		"Path to private key that must be loaded for signing root metadata",
	)

	initCmd.Flags().StringVarP(
		&expires,
		"expires",
		"e",
		"",
		"Expiry for metadata in days",
	)

	initCmd.Flags().StringArrayVarP(
		&publicKeyPaths,
		"public-keys",
		"",
		[]string{},
		"Public keys to be added to metadata",
	)
}

func runInit(cmd *cobra.Command, args []string) error {
	var privKey tufdata.PrivateKey
	privKeyData, err := os.ReadFile(privKeyPath)
	if err != nil {
		return err
	}
	err = json.Unmarshal(privKeyData, &privKey)
	if err != nil {
		return err
	}

	expiresTime, err := parseExpires(expires)
	if err != nil {
		return err
	}

	var publicKeys []tufdata.PublicKey
	for _, publicKeyPath := range publicKeyPaths {
		var pubKey tufdata.PublicKey
		pubKeyData, err := os.ReadFile(publicKeyPath)
		if err != nil {
			return err
		}
		err = json.Unmarshal(pubKeyData, &pubKey)
		if err != nil {
			return err
		}
		publicKeys = append(publicKeys, pubKey)
	}

	rootRoleMb, err := gittuf.Init(privKey, expiresTime, publicKeys)
	if err != nil {
		return err
	}

	rootRoleJson, err := json.Marshal(rootRoleMb)
	if err != nil {
		return err
	}

	// TODO: Embed in Git
	err = os.WriteFile("root.json", rootRoleJson, 0644)
	if err != nil {
		return err
	}

	return nil
}
