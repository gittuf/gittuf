// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package getgithubuserid

import (
	"fmt"
	"os"

	ghutils "github.com/gittuf/gittuf/internal/utils/github"
	"github.com/spf13/cobra"
)

type options struct {
	githubToken string
	githubURL   string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.githubToken,
		"github-token",
		"",
		"token for the GitHub API",
	)

	cmd.Flags().StringVar(
		&o.githubURL,
		"github-url",
		"https://github.com",
		"URL of the GitHub instance to query")
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	if o.githubToken == "" {
		o.githubToken = os.Getenv("GITHUB_TOKEN")
	}

	githubClient, err := ghutils.GetGitHubClient(o.githubURL, o.githubToken)
	if err != nil {
		return err
	}

	user, _, err := githubClient.Users.Get(cmd.Context(), args[0])
	if err != nil {
		return err
	}

	fmt.Printf("User ID for user %s is: %d\n", args[0], user.GetID())

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "get-github-user-id",
		Short:             "Gets the user ID of the specified GitHub user",
		Long:              "This command gets the user ID of the specified GitHub user, needed in a user's identity definition if they are to approve pull requests on GitHub and the gittuf GitHub app is used",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
