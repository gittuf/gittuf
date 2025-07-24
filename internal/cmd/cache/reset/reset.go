// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package reset

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
	return repo.ResetCache()
}

func New() *cobra.Command {
	o := &options{}

	cmd := &cobra.Command{
		Use:   "reset",
		Short: "Reset (delete) the local persistent cache",
		Long:  `The 'reset' command deletes the local persistent cache used by gittuf.`,
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
