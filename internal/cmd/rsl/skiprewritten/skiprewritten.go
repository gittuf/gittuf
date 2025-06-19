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
		Long: `Creates an RSL (Repository Signing Log) annotation to skip all reference entries that point to commits which no longer exist in the specified Git reference.

This is typically used when the commit history has been rewritten — for example, after a rebase, squash, or filter operation — and certain references in the RSL are now invalid. By skipping these entries, this command helps maintain the integrity of RSL validation without requiring the user to manually clean up or update rewritten references.

This command should be run from the root of a valid Git repository and requires one argument: the Git ref (e.g., main or a specific branch) that the RSL should be checked against.`,

		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
