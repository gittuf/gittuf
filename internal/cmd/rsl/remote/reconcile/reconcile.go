// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package reconcile

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository()
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
		Long:              `This command checks the local RSL against the specified remote and reconciles the local RSL if needed. If the local RSL doesn't exist or is strictly behind the remote RSL, then the local RSL is updated to match the remote RSL. If the local RSL is ahead of the remote RSL, nothing is updated. Finally, if the local and remote RSLs have diverged, then the local only RSL entries are reapplied over the latest entries in the remote if the local only RSL entries and remote only entries are for different Git references.`,
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
