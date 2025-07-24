// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package pull

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

	return repo.PullRSL(args[0])
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "pull <remote>",
		Short: "Pull RSL from the specified remote",
		Long: `The 'pull' command downloads the Repository Signing Log (RSL) data from a specified remote.
This ensures the local repository has up-to-date RSL entries reflecting trusted state from the remote.
It requires exactly one argument: the name of the remote repository.`,

		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
