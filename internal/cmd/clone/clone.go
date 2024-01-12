// SPDX-License-Identifier: Apache-2.0

package clone

import (
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	branch string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.branch,
		"branch",
		"b",
		"",
		"specify branch to check out",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	var dir string
	if len(args) > 1 {
		dir = args[1]
	}
	_, err := repository.Clone(cmd.Context(), args[0], dir, o.branch)
	return err
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "clone",
		Short:             "Clone repository and its gittuf references",
		Args:              cobra.MinimumNArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
