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
		Long:              "Reconcile your local Repository Signing Log (RSL) with a remote, updating local history only when behind or non-diverged.",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
