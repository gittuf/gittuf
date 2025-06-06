// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package init

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

	return repo.PopulateCache()
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "init",
		Short: "Initialize persistent cache",
		Long:  `The 'init' command initializes the local persistent cache for a gittuf repository, intended to improve performance of gittuf operations. This cache is local-only and is not synchronized with the remote.`,
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
