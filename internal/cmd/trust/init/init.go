// SPDX-License-Identifier: Apache-2.0

package init

import (
	"os"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/repository"
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

	keyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.InitializeRoot(cmd.Context(), keyBytes, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:     "init",
		Short:   "Initialize gittuf root of trust for repository",
		PreRunE: common.CheckIfSigningViable,
		RunE:    o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
