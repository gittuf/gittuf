// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package init

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p *persistent.Options
}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.InitializeRoot(cmd.Context(), signer, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "init",
		Short:             "Initialize gittuf root of trust for repository",
		PreRunE:           common.CheckIfSigningViableWithFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
