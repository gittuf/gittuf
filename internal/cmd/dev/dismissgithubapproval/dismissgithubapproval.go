// SPDX-License-Identifier: Apache-2.0

package dismissgithubapproval

import (
	"fmt"

	repository "github.com/gittuf/gittuf/gittuf"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

type options struct {
	signingKey        string
	baseURL           string
	reviewID          int64
	dismissedApprover string
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
		&o.dismissedApprover,
		"dismiss-approver",
		"",
		"signing key representing approver whose review must be dismissed (path for SSH, gpg:<fingerprint> for GPG) / identity (fulcio:identity::provider)",
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
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := repository.LoadSigner(o.signingKey)
	if err != nil {
		return err
	}

	dismissedApproverKey, err := repository.LoadPublicKey(o.dismissedApprover)
	if err != nil {
		return err
	}

	return repo.DismissGitHubPullRequestApprover(cmd.Context(), signer, o.baseURL, o.reviewID, dismissedApproverKey, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "dismiss-github-approval",
		Short: fmt.Sprintf("Dismiss GitHub pull request approval as an attestation (developer mode only, set %s=1)", dev.DevModeKey),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
