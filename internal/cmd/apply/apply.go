// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	return repo.ApplyPolicy(cmd.Context(), true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "applies work in progress changes to the policy state to the current policy state",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
