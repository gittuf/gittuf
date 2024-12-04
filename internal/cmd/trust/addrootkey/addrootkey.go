// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addrootkey

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
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
	repo, err := gittuf.LoadRepository()
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

	return repo.AddRootKey(cmd.Context(), signer, newRootKey, true)
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:               "add-root-key",
		Short:             "Add Root key to gittuf root of trust",
		PreRunE:           common.CheckForSigningKeyFlag,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
