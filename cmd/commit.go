package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/adityasaky/gittuf/gittuf"
	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	tufdata "github.com/theupdateframework/go-tuf/data"
)

var commitCmd = &cobra.Command{
	Use:   "commit",
	Short: "Creates a commit object",
	RunE:  runCommit,
	Args:  cobra.MinimumNArgs(1),
}

func init() {
	rootCmd.AddCommand(commitCmd)

	commitCmd.Flags().StringVarP(
		&role,
		"role",
		"",
		"",
		"Targets role to record commit in",
	)

	commitCmd.Flags().StringArrayVarP(
		&roleKeyPaths,
		"role-key",
		"",
		[]string{},
		"Path to signing key for role",
	)

}

func runCommit(cmd *cobra.Command, args []string) error {
	var roleKeys []tufdata.PrivateKey
	for _, k := range roleKeyPaths {
		logrus.Debug("Loading key from", k)
		privKey, err := gittuf.LoadEd25519PrivateKeyFromSslib(k)
		if err != nil {
			return err
		}
		roleKeys = append(roleKeys, privKey)
	}

	newRoleMb, err := gittuf.Commit(role, roleKeys, args...)
	if err != nil {
		return err
	}

	newRoleJson, err := json.Marshal(newRoleMb)
	if err != nil {
		return err
	}

	err = os.WriteFile(
		fmt.Sprintf("%s%s%s.json", gittuf.METADATADIR, string(filepath.Separator), role),
		newRoleJson,
		0644,
	)

	return err
}
