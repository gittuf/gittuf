// SPDX-License-Identifier: Apache-2.0

package githubappapprovals

import (
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	p       *persistent.Options
	enable  bool
	disable bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&o.enable,
		"enable",
		false,
		"mark GitHub app approvals as trusted",
	)

	cmd.Flags().BoolVar(
		&o.disable,
		"disable",
		false,
		"mark GitHub app approvals as untrusted",
	)

	cmd.MarkFlagsOneRequired("enable", "disable") //nolint:errcheck
	cmd.MarkFlagsMutuallyExclusive("enable", "disable")
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(o.p.SigningKey)
	if err != nil {
		return err
	}

	if o.enable {
		return repo.TrustGitHubApp(cmd.Context(), signer, true)
	}

	return repo.UntrustGitHubApp(cmd.Context(), signer, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "github-app-approvals",
		Short:             "Manage whether approvals recorded by the GitHub app are trusted",
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
