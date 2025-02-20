// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dismissapproval

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	githubopts "github.com/gittuf/gittuf/experimental/gittuf/options/github"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p                 *persistent.Options
	baseURL           string
	reviewID          int64
	dismissedApprover string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.baseURL,
		"base-URL",
		githubopts.DefaultGitHubBaseURL,
		"location of GitHub instance",
	)

	cmd.Flags().StringVar(
		&o.dismissedApprover,
		"dismiss-approver",
		"",
		"identity of the reviewer whose review was dismissed",
	)
	cmd.MarkFlagRequired("dismiss-approver") //nolint:errcheck

	cmd.Flags().Int64Var(
		&o.reviewID,
		"review-ID",
		-1,
		"pull request review ID",
	)
	cmd.MarkFlagRequired("review-ID") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.DismissGitHubPullRequestApprover(cmd.Context(), signer, o.reviewID, o.dismissedApprover, true, githubopts.WithGitHubBaseURL(o.baseURL))
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "dismiss-approval",
		Short: "Record dismissal of GitHub pull request approval",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
