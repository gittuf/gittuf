// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package pullrequest

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	githubopts "github.com/gittuf/gittuf/experimental/gittuf/options/github"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p                 *persistent.Options
	baseURL           string
	repository        string
	pullRequestNumber int
	commitID          string
	baseBranch        string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.baseURL,
		"base-URL",
		githubopts.DefaultGitHubBaseURL,
		"location of GitHub instance",
	)

	cmd.Flags().StringVar(
		&o.repository,
		"repository",
		"",
		"path to base GitHub repository the pull request is opened against, of form {owner}/{repo}",
	)
	cmd.MarkFlagRequired("repository") //nolint:errcheck

	cmd.Flags().IntVar(
		&o.pullRequestNumber,
		"pull-request-number",
		-1,
		"pull request number to record in attestation",
	)

	cmd.Flags().StringVar(
		&o.commitID,
		"commit",
		"",
		"commit to record pull request attestation for",
	)

	cmd.Flags().StringVar(
		&o.baseBranch,
		"base-branch",
		"",
		"base branch for pull request, used with --commit",
	)

	// When we're using commit, we need the base branch to filter through nested
	// pull requests
	cmd.MarkFlagsRequiredTogether("commit", "base-branch")

	cmd.MarkFlagsOneRequired("pull-request-number", "commit")
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repositoryParts := strings.Split(o.repository, "/")
	if len(repositoryParts) != 2 {
		return fmt.Errorf("invalid format for repository, must be {owner}/{repo}")
	}

	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []githubopts.Option{githubopts.WithGitHubBaseURL(o.baseURL)}
	if o.p.WithRSLEntry {
		opts = append(opts, githubopts.WithRSLEntry())
	}

	if o.commitID != "" {
		return repo.AddGitHubPullRequestAttestationForCommit(cmd.Context(), signer, repositoryParts[0], repositoryParts[1], o.commitID, o.baseBranch, true, opts...)
	}

	return repo.AddGitHubPullRequestAttestationForNumber(cmd.Context(), signer, repositoryParts[0], repositoryParts[1], o.pullRequestNumber, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "pull-request",
		Short: "Record GitHub pull request information as an attestation",
		Long:  `The 'pull-request' command creates an attestation for a GitHub pull request. It supports attesting either by pull request number or a specific commit and its associated base branch. These attestations help verify the origin and legitimacy of code contributions merged via GitHub. The command also supports custom GitHub base URLs for enterprise GitHub instances, with the flag '--base-URL'.`,
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
