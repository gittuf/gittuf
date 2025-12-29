// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package getgithubappkey

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

	key, err := repo.GetGitHubAppPrincipals(cmd.Context())
	if err != nil {
		return err
	}

	for appName, appPrincipals := range key {
		fmt.Printf("GitHub App Key: (%s)%s\n", appName, appPrincipals)
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "get-github-app-key",
		Short:             "Get the current defined keys for GitHub Apps",
		Long:              "Get the current defined keys for GitHub Apps from the repository's policy.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
