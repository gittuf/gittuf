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
		Use:               "skip-rewritten",
		Short:             "Creates an RSL annotation to skip RSL reference entries that point to commits that do not exist in the specified ref",
		Long:              `The 'skip-rewritten' command adds an RSL annotation to skip reference entries that point to commits no longer present in the given Git reference, useful when the history of a branch has been rewritten and some RSL entries refer to commits that no longer exist on the branch.`,
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
