// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package discard

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	return repo.DiscardPolicy(cmd.Context())
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "discard",
		Short: "Validate and discard the  changes from policy-staging to policy",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
