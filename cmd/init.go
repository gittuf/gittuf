package cmd

import (
	"encoding/json"
	"fmt"
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
	rootPrivKeyPath    string
	targetsPrivKeyPath string
	rootExpires        string
	targetsExpires     string
	publicKeyPaths     []string
)

func init() {
	rootCmd.AddCommand(initCmd)

	initCmd.Flags().StringVarP(
		&rootPrivKeyPath,
		"root-key",
		"",
		"",
		"Path to private key that must be loaded for signing root metadata",
	)

	initCmd.Flags().StringVarP(
		&targetsPrivKeyPath,
		"targets-key",
		"",
		"",
		"Path to private key that must be loaded for signing targets metadata",
	)

	initCmd.Flags().StringVarP(
		&rootExpires,
		"root-expires",
		"",
		"",
		"Expiry for root metadata in days",
	)

	initCmd.Flags().StringVarP(
		&targetsExpires,
		"targets-expires",
		"",
		"",
		"Expiry for targets metadata in days",
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
	rootPrivKey, err := gittuf.LoadEd25519PrivateKeyFromSslib(rootPrivKeyPath)
	if err != nil {
		return err
	}

	targetsPrivKey, err := gittuf.LoadEd25519PrivateKeyFromSslib(
		targetsPrivKeyPath)
	if err != nil {
		return err
	}

	rootExpiresTime, err := parseExpires(rootExpires)
	if err != nil {
		return err
	}

	targetsExpiresTime, err := parseExpires(targetsExpires)
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
	rootPubKey, err := gittuf.GetEd25519PublicKeyFromPrivateKey(&rootPrivKey)
	if err != nil {
		return err
	}
	targetsPubKey, err := gittuf.GetEd25519PublicKeyFromPrivateKey(
		&targetsPrivKey)
	if err != nil {
		return err
	}
	publicKeys = append(publicKeys, rootPubKey, targetsPubKey)

	roles, err := gittuf.Init(
		rootPrivKey,
		rootExpiresTime,
		publicKeys,
		targetsPrivKey,
		targetsExpiresTime)
	if err != nil {
		return err
	}

	for k, v := range roles {
		roleJson, err := json.Marshal(v)
		if err != nil {
			return err
		}
		// TODO: Embed in Git
		err = os.WriteFile(fmt.Sprintf("%s.json", k), roleJson, 0644)
		if err != nil {
			return err
		}
	}

	return nil
}
