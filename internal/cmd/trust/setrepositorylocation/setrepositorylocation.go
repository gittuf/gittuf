// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package setrepositorylocation

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p        *persistent.Options
	location string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.location,
		"location",
		"",
		"canonical location of the repository to record in the root of trust",
	)
	cmd.MarkFlagRequired("location") //nolint:errcheck
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
	return repo.SetRepositoryLocation(cmd.Context(), signer, o.location, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "set-repository-location",
		Short:             "Set repository location",
		Long:              "The 'set-repository-location' command records the canonical location of the repository in its root of trust. It is used to tell other repositories in a gittuf network where to fetch this repository from.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
