// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package enablegithubappapprovals

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	p       *persistent.Options
	appName string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.appName,
		"app-name",
		tuf.GitHubAppRoleName,
		"name of app to add to root of trust",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.TrustGitHubApp(cmd.Context(), signer, o.appName, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "enable-github-app-approvals",
		Short: "Mark GitHub app approvals as trusted henceforth",
		Long: `The 'enable-github-app-approvals' command allows users to mark a GitHub App as trusted in a gittuf-secured Git repository, enabling it to approve protected operations according to the repository’s trust policy.

In gittuf, trust policies govern which actors—such as developers, maintainers, or automated systems—are authorized to perform critical actions. GitHub Apps can be included in these policies to automate and securely manage approvals, such as for pull requests, merges, or releases.

This command registers the specified GitHub App as a trusted approver by adding it to the root of trust. The app’s name must be provided using the '--app-name' flag; by default, this is set to the conventional role used by gittuf, but it can be customized as needed.

To authorize this trust modification, the user must provide a valid signing key via the persistent '--signing-key' flag. Optionally, the '--rsl-entry' flag may be included to log the trust change in the Reference State Log (RSL), providing a verifiable audit trail of trust policy updates.

This command is essential for securely integrating GitHub Apps into the approval workflow of secure software supply chains, especially in CI/CD environments, automated governance setups, or multi-party review systems.`,

		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
