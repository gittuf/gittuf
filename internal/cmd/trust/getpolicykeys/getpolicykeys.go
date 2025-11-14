// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package getpolicykeys

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

	principals, err := repo.GetPrimaryRuleFilePrincipals(cmd.Context())
	if err != nil {
		return err
	}

	for _, principal := range principals {
		fmt.Printf("Principal trusted: %s\n", principal.ID())
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "get-policy-key",
		Short:             "Get the current policy key",
		Long:              "Get the current policy key from the repository's policy.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
