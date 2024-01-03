// SPDX-License-Identifier: Apache-2.0

package addrootkey

import (
	"log/slog"
	"os"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/repository"
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
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	rootKeyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}

	newRootKeyBytes, err := common.ReadKeyBytes(o.newRootKey)
	if err != nil {
		return err
	}

	err = repo.AddRootKey(cmd.Context(), rootKeyBytes, newRootKeyBytes, true)
	if err != nil {
		return err
	}
	slog.Info("Added authorized key for root role")

	return nil
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:     "add-root-key",
		Short:   "Add Root key to gittuf root of trust",
		PreRunE: common.CheckIfSigningViable,
		RunE:    o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
