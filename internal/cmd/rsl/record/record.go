// SPDX-License-Identifier: Apache-2.0

package record

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/repository"
	rslopts "github.com/gittuf/gittuf/internal/repository/options/rsl"
	verifyopts "github.com/gittuf/gittuf/internal/repository/options/verify"
	"github.com/spf13/cobra"
)

type options struct {
	dstRef    string
	fromEntry string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.dstRef,
		"dst-ref",
		"",
		"name of destination reference, if it differs from source reference",
	)
	cmd.Flags().StringVar(
		&o.fromEntry,
		"from-entry",
		"",
		"perform verification from specified RSL entry",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	if err := repo.RecordRSLEntryForReference(args[0], true, rslopts.WithOverrideRefName(o.dstRef)); err != nil {
		return err
	}

	if o.fromEntry == "" {
		err = repo.VerifyRef(cmd.Context(), args[0], verifyopts.WithOverrideRefName(o.dstRef))
	} else {
		err = repo.VerifyRefFromEntry(cmd.Context(), args[0], o.fromEntry)
	}

	if err != nil {
		if err := repo.RollbackLatestRSLEntry(); err != nil {
			return fmt.Errorf("removal of the RSL entry failed for %s: %w", args[0], err)
		}
		return fmt.Errorf("policy verification failed for %s: %w", args[0], err)
	}
	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "record",
		Short:             "Record latest state of a Git reference in the RSL after verifying against gittuf policy",
		Args:              cobra.ExactArgs(1),
		PreRunE:           common.CheckIfSigningViable,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
