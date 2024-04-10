// SPDX-License-Identifier: Apache-2.0

package rslrecordat

import (
	"fmt"
	"os"

	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/repository"
	rslopts "github.com/gittuf/gittuf/internal/repository/options/rsl"
	verifyopts "github.com/gittuf/gittuf/internal/repository/options/verify"
	"github.com/spf13/cobra"
)

type options struct {
	targetID       string
	signingKeyPath string
	dstRef         string
	fromEntry      string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.dstRef,
		"dst-ref",
		"",
		"name of destination reference, if it differs from source reference",
	)

	cmd.Flags().StringVarP(
		&o.targetID,
		"target",
		"t",
		"",
		"target ID",
	)
	cmd.MarkFlagRequired("target") //nolint:errcheck

	cmd.Flags().StringVarP(
		&o.signingKeyPath,
		"signing-key",
		"k",
		"",
		"path to PEM encoded SSH or GPG signing key",
	)
	cmd.MarkFlagRequired("signing-key") //nolint:errcheck

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

	signingKeyBytes, err := os.ReadFile(o.signingKeyPath)
	if err != nil {
		return err
	}

	if err := repo.RecordRSLEntryForReferenceAtTarget(args[0], o.targetID, signingKeyBytes, rslopts.WithOverrideRefName(o.dstRef)); err != nil {
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
		Use:   "rsl-record",
		Short: fmt.Sprintf("Record explicit state of a Git reference in the RSL, if it passes verification, signed with specified key (developer mode only, set %s=1)", dev.DevModeKey),
		Args:  cobra.ExactArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
