// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	signingKey string
	fromRef    string
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

	cmd.Flags().StringVarP(
		&o.fromRef,
		"from-ref",
		"f",
		"",
		"ref to authorize merging changes from",
	)
	cmd.MarkFlagRequired("from-ref") //nolint:errcheck

	cmd.Flags().BoolVarP(
		&o.revoke,
		"revoke",
		"r",
		false,
		"revoke existing authorization",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	signer, err := common.LoadSigner(o.signingKey)
	if err != nil {
		return err
	}

	if o.revoke {
		if len(args) < 3 {
			return fmt.Errorf("insufficient parameters for revoking authorization, requires <targetRef> <fromID> <targetTreeID>")
		}

		return repo.RemoveReferenceAuthorization(cmd.Context(), signer, args[0], args[1], args[2], true)
	}

	return repo.AddReferenceAuthorization(cmd.Context(), signer, args[0], o.fromRef, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "authorize",
		Short:             fmt.Sprintf("Add or revoke reference authorization (developer mode only, set %s=1)", dev.DevModeKey),
		Args:              cobra.MinimumNArgs(1),
		PreRunE:           common.CheckIfSigningViable,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
