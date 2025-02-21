// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removepropagationdirective

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

type options struct {
	p    *persistent.Options
	name string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.name,
		"name",
		"",
		"name of propagation directive",
	)
	cmd.MarkFlagRequired("name") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	repo, err := gittuf.LoadRepository()
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
	return repo.RemovePropagationDirective(cmd.Context(), signer, o.name, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "remove-propagation-directive",
		Short:             fmt.Sprintf("Remove propagation directive from gittuf root of trust (developer mode only, set %s=1)", dev.DevModeKey),
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
