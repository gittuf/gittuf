// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package getgithubappapprovalsstatus

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

	approvals, err := repo.AreGitHubAppApprovalsTrusted(cmd.Context())
	if err != nil {
		return err
	}

	for appName, isTrusted := range approvals {
		if isTrusted {
			fmt.Printf("GitHub App approvals for %s: Trusted\n", appName)
		} else {
			fmt.Printf("GitHub App approvals for %s: Untrusted\n", appName)
		}
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "get-github-app-approvals-status",
		Short:             "Get whether GitHub App approvals are trusted",
		Long:              "Get whether GitHub App approvals are trusted for each GitHub app, from the repository's policy.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
