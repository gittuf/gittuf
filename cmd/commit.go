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

var (
	roleExpires string
)

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

	commitCmd.Flags().StringVarP(
		&roleExpires,
		"role-expires",
		"",
		"",
		"Expiry for role metadata in days",
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

	expires, err := parseExpires(roleExpires, "targets")
	if err != nil {
		return err
	}

	newRoleMb, err := gittuf.Commit(repo, role, roleKeys, expires, args...)
	if err != nil {
		return err
	}

	// All errors after this point should undo the commit

	newRoleBytes, err := json.Marshal(newRoleMb)
	if err != nil {
		return gittuf.UndoCommit(err)
	}

	err = repo.StageAndCommit(role, newRoleBytes)
	if err != nil {
		return gittuf.UndoCommit(err)
	}

	return nil
}
