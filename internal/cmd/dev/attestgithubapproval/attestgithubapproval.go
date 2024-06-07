// SPDX-License-Identifier: Apache-2.0

package attestgithubapproval

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	signingKey string
	baseBranch string
	fromID     string
	toID       string
	approver   string
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
		&o.baseBranch,
		"base-branch",
		"",
		"base branch for pull request",
	)
	cmd.MarkFlagRequired("base-branch") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.fromID,
		"from",
		"",
		"`from` revision ID--current tip of the base branch",
	)
	cmd.MarkFlagRequired("from") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.toID,
		"to",
		"",
		"`to` tree ID--the resultant Git tree when this pull request is merged",
	)
	cmd.MarkFlagRequired("to") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.approver,
		"approver",
		"",
		"approver signing key (path for SSH, gpg:<fingerprint> for GPG) / identity (fulcio:identity::provider)",
	)
	cmd.MarkFlagRequired("approver") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(o.signingKey)
	if err != nil {
		return err
	}

	approverKey, err := common.LoadPublicKey(o.approver)
	if err != nil {
		return err
	}

	return repo.AddGitHubPullRequestApprovalAttestation(cmd.Context(), signer, o.baseBranch, o.fromID, o.toID, approverKey, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "attest-github-approval",
		Short: fmt.Sprintf("Record GitHub pull request approval as an attestation (developer mode only, set %s=1)", dev.DevModeKey),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
