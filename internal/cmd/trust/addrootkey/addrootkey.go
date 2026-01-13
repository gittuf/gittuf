// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addrootkey

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/spf13/cobra"
)

type options struct {
	p          *persistent.Options
	newRootKey string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.newRootKey,
		"root-key",
		"",
		"root key to add to root of trust",
	)
	cmd.MarkFlagRequired("root-key") //nolint:errcheck
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

	newRootKey, err := gittuf.LoadPublicKey(o.newRootKey)
	if err != nil {
		return err
	}

	opts := []trustpolicyopts.Option{}
	if o.p.WithRSLEntry {
		opts = append(opts, trustpolicyopts.WithRSLEntry())
	}
	return repo.AddRootKey(cmd.Context(), signer, newRootKey, true, opts...)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-root-key",
		Short:             "Add Root key to gittuf root of trust",
		Long:              `The 'add-root-key' command allows users to add a new root key to the repository's root of trust. This command facilitates the addition of an extra root key to the existing trusted root keys, enabling multiple root keys or key rotation. It requires a public key file or Sigstore identity via --root-key and a signing key via --signing-key. Optionally, the change can be recorded in the RSL.`,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
