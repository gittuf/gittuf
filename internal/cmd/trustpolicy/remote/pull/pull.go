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

	return repo.PullPolicy(args[0])
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "pull <remote>",
		Short:             "Pull policy from the specified remote",
		Long:              "The 'pull' command retrieves policy updates from the specified remote and applies them to the local repository.",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
