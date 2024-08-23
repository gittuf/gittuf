// SPDX-License-Identifier: Apache-2.0

package push

import (
	repository "github.com/gittuf/gittuf/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
}

func (o *options) Run(_ *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	return repo.PushPolicy(args[0])
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "push <remote>",
		Short:             "Push policy to the specified remote",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
