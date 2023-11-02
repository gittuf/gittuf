// SPDX-License-Identifier: Apache-2.0

package add

import (
	"os"

	"github.com/gittuf/gittuf/internal/cmd/authorize/persistent"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p           *persistent.Options
	fromRefName string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.fromRefName,
		"from",
		"f",
		"",
		"Specify source ref for authorization",
	)
	cmd.MarkFlagRequired("from") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.AddAuthorization(cmd.Context(), args[0], o.fromRefName, keyBytes, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "add <refName>",
		Short: "Add authorization for updating the state of a ref",
		Args:  cobra.ExactArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
