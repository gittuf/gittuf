// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package discard

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(_ *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	return repo.DiscardPolicy()
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "discard",
		Short:             "Discard the currently staged changes to policy",
		Long:              "The 'discard' command removes any currently staged policy changes. It is used to revert pending policy updates before they are applied to the repository.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
