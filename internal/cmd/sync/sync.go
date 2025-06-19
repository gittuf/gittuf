// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/spf13/cobra"
)

type options struct {
	overwriteLocalRefs bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&o.overwriteLocalRefs,
		"overwrite",
		false,
		"overwrite local references with upstream changes",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	remoteName := gitinterface.DefaultRemoteName
	if len(args) > 0 {
		remoteName = args[0]
	}

	divergedRefs, err := repo.Sync(cmd.Context(), remoteName, o.overwriteLocalRefs, true)
	switch {
	case errors.Is(err, gittuf.ErrDivergedRefs):
		fmt.Println("References have diverged:")
		for _, refName := range divergedRefs {
			fmt.Println(refName)
		}
		fmt.Println("To apply upstream changes locally, rerun the command with --overwrite. WARNING: this operation may result in the loss of local work!")
		return nil
	default:
		return err
	}
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "sync [remoteName]",
		Short: "Synchronize local references with remote references based on RSL",
		Long: `Synchronizes local references with the remote repository using information recorded in the Repository Signing Log (RSL).

This command compares the references in the local Git repository with those from the specified remote (defaults to 'origin' if not provided) and updates the local repository to match the upstream state. It helps ensure that both repositories are cryptographically aligned according to the trusted RSL entries.

If any references have diverged—meaning the local and remote histories differ—these will be listed without modifying them. To forcefully update the local references and overwrite local changes, the --overwrite flag must be used. WARNING: using --overwrite may result in loss of unmerged or uncommitted local work.

This command is useful when maintaining RSL consistency across collaborators or machines in a secure supply chain setup.`,

		Args: cobra.MaximumNArgs(1),
		RunE: o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
