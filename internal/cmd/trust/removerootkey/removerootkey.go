// SPDX-License-Identifier: Apache-2.0

package removerootkey

import (
	"log/slog"
	"os"
	"strings"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/repository"
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
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	rootKeyBytes, err := os.ReadFile(o.p.SigningKey)
	if err != nil {
		return err
	}

	err = repo.RemoveRootKey(cmd.Context(), rootKeyBytes, strings.ToLower(o.rootKeyID), true)
	if err != nil {
		return err
	}
	slog.Info("Removed authorized key for root role", "keyID", o.rootKeyID)

	return nil
}

func New(persistent *persistent.Options) *cobra.Command {
	o := &options{p: persistent}
	cmd := &cobra.Command{
		Use:     "remove-root-key",
		Short:   "Remove Root key from gittuf root of trust",
		PreRunE: common.CheckIfSigningViable,
		RunE:    o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
