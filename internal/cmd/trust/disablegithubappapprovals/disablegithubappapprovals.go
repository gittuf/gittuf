// SPDX-License-Identifier: Apache-2.0

package disablegithubappapprovals

import (
	repository "github.com/gittuf/gittuf/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p *persistent.Options
}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := repository.LoadSigner(o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.UntrustGitHubApp(cmd.Context(), signer, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "disable-github-app-approvals",
		Short:             "Mark GitHub app approvals as untrusted henceforth",
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
