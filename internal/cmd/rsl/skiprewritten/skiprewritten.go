// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package skiprewritten

import (
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(_ *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
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
		Args:              cobra.ExactArgs(1),
		PreRunE:           common.CheckIfSigningViable,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
