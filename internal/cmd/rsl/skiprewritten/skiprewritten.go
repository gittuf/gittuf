// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package skiprewritten

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(_ *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	return repo.SkipAllInvalidReferenceEntriesForRef(args[0], true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "skip-rewritten",
		Short: "Creates an RSL annotation to skip RSL reference entries that point to commits that do not exist in the specified ref",
		Long:  "Annotate the RSL to skip entries for rewritten commits that no longer exist in the specified Git reference.",

		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
