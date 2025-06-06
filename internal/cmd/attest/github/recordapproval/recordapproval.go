// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package recordapproval

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
	reviewID          int64
	approver          string
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
		"pull request number",
	)
	cmd.MarkFlagRequired("pull-request-number") //nolint:errcheck

	cmd.Flags().Int64Var(
		&o.reviewID,
		"review-ID",
		-1,
		"pull request review ID",
	)
	cmd.MarkFlagRequired("review-ID") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.approver,
		"approver",
		"",
		"identity of the reviewer who approved the change",
	)
	cmd.MarkFlagRequired("approver") //nolint:errcheck
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

	return repo.AddGitHubPullRequestApprover(cmd.Context(), signer, repositoryParts[0], repositoryParts[1], o.pullRequestNumber, o.reviewID, o.approver, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "record-approval",
		Short: "Record GitHub pull request approval",
		Long:  `The 'record-approval' command creates an attestation for an approval action on a GitHub pull request. This command requires the repository in the {owner}/{repo} format, the pull request number, the specific review ID, and the identity of the reviewer who approved the pull request. The command also supports custom GitHub base URLs for enterprise GitHub instances, with the flag '--base-URL'.`,
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
