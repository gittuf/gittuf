package cmd

import (
	"encoding/json"

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
	repo, err := getGittufRepo()
	if err != nil {
		return err
	}
	var roleKeys []tufdata.PrivateKey
	for _, k := range roleKeyPaths {
		logrus.Debug("Loading key from", k)
		privKey, err := gittuf.LoadEd25519PrivateKeyFromSslib(k)
		if err != nil {
			return err
		}
		roleKeys = append(roleKeys, privKey)
	}

	newRoleMb, err := gittuf.Commit(repo, role, roleKeys, args...)
	if err != nil {
		return err
	}

	// TODO: All commits after this should undo the commit

	newRoleBytes, err := json.Marshal(newRoleMb)
	if err != nil {
		return err
	}

	return repo.StageAndCommit(role+".json", newRoleBytes)
}
