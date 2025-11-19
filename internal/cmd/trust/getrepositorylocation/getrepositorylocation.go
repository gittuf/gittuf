// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package getrepositorylocation

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	// "github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	rule, err := repo.GetRepositoryLocation(cmd.Context())
	if err != nil {
		return err
	}

	fmt.Printf("Repository Location: %s\n", rule)
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "get-repository-location",
		Short:             "Get the current repository location",
		Long:              "Get the current repository location from the repository's policy.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
