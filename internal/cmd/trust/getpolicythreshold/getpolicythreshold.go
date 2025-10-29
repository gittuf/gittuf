// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package getpolicythreshold

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

	// Fetch the current policy threshold.
	threshold, err := repo.GetTopLevelTargetsThreshold(cmd.Context())
	if err != nil {
		return err
	}

	fmt.Printf("Current policy threshold: %d\n", threshold)
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "get-policy-threshold",
		Short:             "List the currently defined global rules for the root of trust",
		Long:              "List the currently defined global rules for the root of trust",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
