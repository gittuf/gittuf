// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addpropagationdirective

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
	p                   *persistent.Options
	name                string
	upstreamRepository  string
	upstreamReference   string
	upstreamPath        string
	downstreamReference string
	downstreamPath      string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.name,
		"name",
		"",
		"name of propagation directive",
	)
	cmd.MarkFlagRequired("name") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.upstreamRepository,
		"from-repository",
		"",
		"location of upstream repository",
	)
	cmd.MarkFlagRequired("from-repository") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.upstreamReference,
		"from-reference",
		"",
		"reference to propagate from in upstream repository",
	)
	cmd.MarkFlagRequired("from-reference") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.upstreamPath,
		"from-path",
		"",
		"path in upstream reference to propagate contents from",
	)

	cmd.Flags().StringVar(
		&o.downstreamReference,
		"into-reference",
		"",
		"reference to propagate into in downstream repository",
	)
	cmd.MarkFlagRequired("into-reference") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.downstreamPath,
		"into-path",
		"",
		"path to propagate upstream contents into in downstream reference",
	)
	cmd.MarkFlagRequired("into-path") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

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
	return repo.AddPropagationDirective(cmd.Context(), signer, o.name, o.upstreamRepository, o.upstreamReference, o.upstreamPath, o.downstreamReference, o.downstreamPath, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-propagation-directive",
		Short:             fmt.Sprintf("Add propagation directive into gittuf root of trust (developer mode only, set %s=1)", dev.DevModeKey),
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
