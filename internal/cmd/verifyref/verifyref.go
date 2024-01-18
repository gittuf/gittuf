// SPDX-License-Identifier: Apache-2.0

package verifyref

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/featureflags"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct {
	latestOnly         bool
	fromEntry          string
	usePolicyPathCache bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&o.latestOnly,
		"latest-only",
		false,
		"perform verification against latest entry in the RSL",
	)

	cmd.Flags().StringVar(
		&o.fromEntry,
		"from-entry",
		"",
		fmt.Sprintf("perform verification from specified RSL entry (developer mode only, set %s=1)", dev.DevModeKey),
	)

	cmd.MarkFlagsMutuallyExclusive("latest-only", "from-entry")

	cmd.Flags().BoolVar(
		&o.usePolicyPathCache,
		"use-policy-path-cache",
		false,
		fmt.Sprintf("use policy path cache during verification (developer mode only, set %s=1)", dev.DevModeKey),
	)
}

func (o *options) PreRunE(_ *cobra.Command, _ []string) error {
	if o.usePolicyPathCache {
		if !dev.InDevMode() {
			return dev.ErrNotInDevMode
		}

		featureflags.UsePolicyPathCache = true
	}
	return nil
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	if o.fromEntry != "" {
		if !dev.InDevMode() {
			return dev.ErrNotInDevMode
		}

		return repo.VerifyRefFromEntry(cmd.Context(), args[0], o.fromEntry)
	}

	return repo.VerifyRef(cmd.Context(), args[0], o.latestOnly)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "verify-ref",
		Short:             "Tools for verifying gittuf policies",
		Args:              cobra.ExactArgs(1),
		PreRunE:           o.PreRunE,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
