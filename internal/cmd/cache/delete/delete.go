// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package delete

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
	return repo.DeleteCache()
}

func New() *cobra.Command {
	o := &options{}

	cmd := &cobra.Command{
		Use:   "delete",
		Short: "Delete the local persistent cache",
		Long:  `The 'delete' command deletes the local persistent cache used by gittuf.`,
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
