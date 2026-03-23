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
		Use:               "push <remote>",
		Short:             "Push policy to the specified remote",
		Long:              "The 'push' command sends the repository's policy to the specified remote. It is used to publish local policy changes so that they are available for other users to pull down.",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
