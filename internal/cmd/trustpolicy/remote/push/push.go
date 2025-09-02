// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
}

func (o *options) Run(_ *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	return repo.PushPolicy(args[0])
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "push <remote>",
		Short: "Push policy to the specified remote",
		Long: `Push uploads the local trust policy to the specified remote, ensuring the
remote repository is updated with the latest policy state. The remote name must
be provided as an argument.`,

		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
