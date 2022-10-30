package cmd

import (
	"github.com/adityasaky/gittuf/gittuf"
	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize a Git repository with TUF support",
	RunE:  init,
}

func init() {
	rootCmd.AddCommand(initCmd)

	var privKeyPath string
	initCmd.Flags().StringVarP(
		&privKeyPath,
		"private-key",
		"k",
		"",
		"Path to private key that must be loaded for signing root metadata",
	)

	var expires string
	initCmd.Flags().StringVarP(
		&expires,
		"expires",
		"e",
		"",
		"Expiry for metadata",
	)

	publicKeyPaths := initCmd.Flags().StringSliceP(
		"public-keys",
		"",
		[]string{},
		"Public keys to be added to metadata",
	)

	// TODO: load privKey from path

	// TODO: load public keys to add to root from specified paths

	gittuf.Init(privKey, expires, publicKeys)
}
