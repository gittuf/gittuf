// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addrootkey

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd/common"
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
		Use:   "add-root-key",
		Short: "Add Root key to gittuf root of trust",
		Long: `The 'add-root-key' command allows users to add a new root key to the root of trust in a gittuf-secured Git repository.

In gittuf, the root of trust is initialized with one or more root keys that are used to validate and authorize changes to the repository’s trust metadata. This command facilitates the addition of an extra root key to the existing trusted root keys, enabling a multi-root setup or key rotation.

To perform this operation, the user must specify the new root key file using the '--root-key' flag, which should point to a PEM-encoded public key file. The current user must also provide a signing key via the persistent '--signing-key' flag, as adding a root key is a signed action that updates the trust policy.

Optionally, the '--rsl-entry' flag can be set to indicate that the addition of the new root key should be recorded in the Repository Signing Log (RSL), providing an auditable trail of trust-related changes.

This command is essential for administrative operations such as delegating trust to additional parties, performing key rotations, or implementing key recovery strategies within a secure and verifiable Git ecosystem.`,

		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
