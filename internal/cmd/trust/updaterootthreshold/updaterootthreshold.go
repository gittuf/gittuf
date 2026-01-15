// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package updaterootthreshold

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p         *persistent.Options
	threshold int
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().IntVar(
		&o.threshold,
		"threshold",
		-1,
		"threshold of valid signatures required for root",
	)
	cmd.MarkFlagRequired("threshold") //nolint:errcheck
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
	return repo.UpdateRootThreshold(cmd.Context(), signer, o.threshold, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:   "update-root-threshold",
		Short: "Update Root threshold in the gittuf root of trust",
		Long:  "This command allows users to update the threshold of valid signatures required for the root of trust.",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
