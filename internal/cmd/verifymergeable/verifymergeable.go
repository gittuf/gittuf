// SPDX-License-Identifier: Apache-2.0

package verifymergeable

import (
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	baseBranch string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.baseBranch,
		"into",
		"",
		"base branch to merge into",
	)
	cmd.MarkFlagRequired("into") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	return repo.VerifyMergeable(cmd.Context(), o.baseBranch, args[0])
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "verify-mergeable",
		Short:             "Verify if a branch can be merged into another using gittuf policies",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
