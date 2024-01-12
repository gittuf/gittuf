// SPDX-License-Identifier: Apache-2.0

package pull

import (
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	return repo.PullPolicy(cmd.Context(), args[0])
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "pull <remote>",
		Short:             "Pull policy from the specified remote",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
