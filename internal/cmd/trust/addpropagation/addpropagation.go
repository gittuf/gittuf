// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addpropagation

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

type options struct {
	p                   *persistent.Options
	upstreamRepository  string
	upstreamReference   string
	downstreamReference string
	downstreamPath      string
}

func (o *options) AddFlags(cmd *cobra.Command) {
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

	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := gittuf.LoadSigner(repo, o.p.SigningKey)
	if err != nil {
		return err
	}

	return repo.AddPropagation(cmd.Context(), signer, o.upstreamRepository, o.upstreamReference, o.downstreamReference, o.downstreamPath, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-propagation",
		Short:             fmt.Sprintf("Add propagation directive into gittuf root of trust (developer mode only, set %s=1)", dev.DevModeKey),
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
