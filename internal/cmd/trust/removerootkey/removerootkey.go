// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package removerootkey

import (
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p         *persistent.Options
	rootKeyID string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.rootKeyID,
		"root-key-ID",
		"",
		"ID of Root key to be removed from root of trust",
	)
	cmd.MarkFlagRequired("root-key-ID") //nolint:errcheck
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
	return repo.RemoveRootKey(cmd.Context(), signer, strings.ToLower(o.rootKeyID), true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "remove-root-key",
		Short:             "Remove Root key from gittuf root of trust",
		Long:              "The 'remove-root-key' command removes the specified root key from the repository's gittuf root of trust. This command updates the trust policy to reflect the removal of the provided root key ID, ensuring that the corresponding key is no longer trusted for root of trust operations.",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
