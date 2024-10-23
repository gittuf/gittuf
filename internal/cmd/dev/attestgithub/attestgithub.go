// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestgithub

import (
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

type options struct {
	signingKey        string
	baseURL           string
	repository        string
	pullRequestNumber int
	commitID          string
	baseBranch        string
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

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(repo, o.signingKey)
	if err != nil {
		return err
	}

	if o.commitID != "" {
		return repo.AddGitHubPullRequestAttestationForCommit(cmd.Context(), signer, o.baseURL, repositoryParts[0], repositoryParts[1], o.commitID, o.baseBranch, true)
	}

	return repo.AddGitHubPullRequestAttestationForNumber(cmd.Context(), signer, o.baseURL, repositoryParts[0], repositoryParts[1], o.pullRequestNumber, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "attest-github",
		Short: fmt.Sprintf("Record GitHub pull request information as an attestation (developer mode only, set %s=1)", dev.DevModeKey),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
