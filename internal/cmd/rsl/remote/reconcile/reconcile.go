// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package reconcile

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	return repo.ReconcileLocalRSLWithRemote(cmd.Context(), args[0], true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "reconcile <remote>",
		Short:             "Reconcile local RSL with remote RSL",
		Long:              "The 'reconcile' command checks the local RSL against the specified remote and reconciles the local RSL if needed. It is used to bring the local RSL back in line with the remote after the two diverge. If the local RSL does not exist or is strictly behind the remote, it is updated to match the remote; if it is ahead, nothing is updated; and if the two have diverged, the local-only entries are reapplied over the latest remote entries when the local-only and remote-only entries are for different Git references.",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
