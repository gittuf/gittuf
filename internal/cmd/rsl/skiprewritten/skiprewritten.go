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
		Long: "The skiprewritten command tells gittuf to ignore rewritten commits in the repository.

This is useful in situations where commits are modified by automated tools (e.g., rebase,
formatting tools, squashing, etc.) and you want gittuf to skip verification or enforcement
for those rewritten commits. This command marks such commits explicitly so they are not
subject to the same rules as regular, verified commits.

It is typically used during complex history rewriting where preserving the verification
status of original commits isn't possible or desired.",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
