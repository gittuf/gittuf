// SPDX-License-Identifier: Apache-2.0

package verifyref

import (
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	latestOnly bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&o.latestOnly,
		"latest-only",
		false,
		"perform verification against latest entry in the RSL",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}
	return repo.VerifyRef(cmd.Context(), args[0], o.latestOnly)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "verify-ref",
		Short: "Tools for verifying gittuf policies",
		Args:  cobra.ExactArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
