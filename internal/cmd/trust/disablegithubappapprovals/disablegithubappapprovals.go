// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package disablegithubappapprovals

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
	return repo.UntrustGitHubApp(cmd.Context(), signer, o.appName, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "disable-github-app-approvals",
		Short: "Mark GitHub app approvals as untrusted henceforth",
		Long: `The 'disable-github-app-approvals' command allows users to mark a GitHub App as untrusted in a gittuf-secured Git repository, thereby disabling its ability to approve changes in the future.

GitHub Apps can be integrated into a repository’s trust policy to automatically approve commits, merges, or other protected operations based on their identity. However, there may be situations where a previously trusted GitHub App must be revoked due to a change in ownership, compromised credentials, or a shift in repository governance.

This command enables repository maintainers to explicitly untrust a GitHub App by specifying its name using the '--app-name' flag. By default, the app name is set to the conventional role used by gittuf, but it can be overridden as needed. Once untrusted, the GitHub App will no longer have the authority to approve actions under the repository’s trust policy.

The action requires a signing key, passed using the persistent '--signing-key' flag, to authorize the change. Optionally, if the '--rsl-entry' flag is set, the change will be recorded in the Repository Signing Log (RSL), ensuring that the modification to the trust configuration is auditable and tamper-evident.

This command is useful for managing trust lifecycle events, reducing the risk of unauthorized access, and maintaining the integrity of the approval process within secure Git workflows.`,

		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
