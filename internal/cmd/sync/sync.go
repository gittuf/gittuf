// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sync //nolint:revive

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

	stdOut := cmd.OutOrStdout()

	divergedRefs, err := repo.Sync(cmd.Context(), remoteName, o.overwriteLocalRefs, true)
	switch {
	case errors.Is(err, gittuf.ErrDivergedRefs):
		fmt.Fprintln(stdOut, "References have diverged:")
		for _, refName := range divergedRefs {
			fmt.Fprintln(stdOut, refName)
		}
		fmt.Fprintln(stdOut, "To apply upstream changes locally, rerun the command with --overwrite. WARNING: this operation may result in the loss of local work!")
		return nil
	default:
		return err
	}
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "sync [remoteName]",
		Short:             "Synchronize local references with remote references based on RSL",
		Long:              "The 'sync' command synchronizes the local repository with the remote by fetching and applying the latest gittuf metadata. It is used to ensure the local state is up to date with what has been pushed to the remote.",
		Args:              cobra.MaximumNArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
