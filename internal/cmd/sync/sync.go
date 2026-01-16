// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
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
		Long:  `The 'sync' command synchronizes local references with the remote references based on the RSL (Reference State Log). By default, it uses the 'origin' remote unless a different remote name is provided. If references have diverged, it prints the list of affected refs and suggests rerunning the command with --overwrite to apply remote changes. Use with caution: --overwrite may discard local changes.`,
		Args:  cobra.MaximumNArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
