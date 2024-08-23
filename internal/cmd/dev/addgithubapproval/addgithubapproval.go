// SPDX-License-Identifier: Apache-2.0

package addgithubapproval

import (
	"fmt"
	"strings"

	repository "github.com/gittuf/gittuf/gittuf"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

type options struct {
	signingKey        string
	baseURL           string
	repository        string
	pullRequestNumber int
	reviewID          int64
	approver          string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.signingKey,
		"signing-key",
		"k",
		"",
		"signing key to use for signing attestation",
	)
	cmd.MarkFlagRequired("signing-key") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.baseURL,
		"base-URL",
		"https://github.com",
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
		"approver signing key (path for SSH, gpg:<fingerprint> for GPG) / identity (fulcio:identity::provider)",
	)
	cmd.MarkFlagRequired("approver") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repositoryParts := strings.Split(o.repository, "/")
	if len(repositoryParts) != 2 {
		return fmt.Errorf("invalid format for repository, must be {owner}/{repo}")
	}

	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := repository.LoadSigner(o.signingKey)
	if err != nil {
		return err
	}

	approverKey, err := repository.LoadPublicKey(o.approver)
	if err != nil {
		return err
	}

	return repo.AddGitHubPullRequestApprover(cmd.Context(), signer, o.baseURL, repositoryParts[0], repositoryParts[1], o.pullRequestNumber, o.reviewID, approverKey, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "add-github-approval",
		Short: fmt.Sprintf("Record GitHub pull request approval as an attestation (developer mode only, set %s=1)", dev.DevModeKey),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
