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
		Use:   "pull <remote>",
		Short: "Pull policy from the specified remote",
		Long: `Pull fetches the trust policy from the specified remote and updates the
local repository. This ensures the local policy is synchronized with the
version maintained on the remote. The remote name must be provided as an
argument.`,

		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
