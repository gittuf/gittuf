// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"fmt"
	"os"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	signingKey string
	revoke     bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.signingKey,
		"signing-key",
		"k",
		"",
		"signing key to use for creating or revoking an authorization",
	)
	cmd.MarkFlagRequired("signing-key") //nolint:errcheck

	cmd.Flags().BoolVarP(
		&o.revoke,
		"revoke",
		"r",
		false,
		"revoke existing authorization",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	keyBytes, err := os.ReadFile(o.signingKey)
	if err != nil {
		return err
	}

	if o.revoke {
		if len(args) < 3 {
			return fmt.Errorf("insufficient parameters for revoking authorization, requires <targetRef> <fromID> <toID>")
		}

		return repo.RemoveReferenceAuthorization(keyBytes, args[0], args[1], args[2], true)
	}

	return repo.AddReferenceAuthorization(cmd.Context(), keyBytes, args[0], true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "authorize",
		Short:             "Add or revoke reference authorization",
		Args:              cobra.MinimumNArgs(1),
		PreRunE:           common.CheckIfSigningViable,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
