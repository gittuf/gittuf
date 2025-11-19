// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package getrootthreshold

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	threshold, err := repo.GetRootThreshold(cmd.Context())
	if err != nil {
		return err
	}

	fmt.Printf("Current root threshold: %d\n", threshold)
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "get-root-threshold",
		Short:             "Get the current root threshold",
		Long:              "Get the current root threshold from the repository's policy.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
