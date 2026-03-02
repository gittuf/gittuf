// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifyref

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	verifyopts "github.com/gittuf/gittuf/experimental/gittuf/options/verify"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

type options struct {
	latestOnly    bool
	repairIssues  bool
	fromEntry     string
	remoteRefName string
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

	cmd.Flags().StringVar(
		&o.remoteRefName,
		"remote-ref-name",
		"",
		"name of remote reference, if it differs from the local name",
	)

	cmd.Flags().BoolVar(
		&o.repairIssues,
		"repair-issues",
		false,
		"perform automated recovery of verification issues",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	opts := []verifyopts.Option{verifyopts.WithOverrideRefName(o.remoteRefName)}
	if o.repairIssues {
		opts = append(opts, verifyopts.WithAutomatedRecovery())
	}

	if o.fromEntry != "" {
		if !dev.InDevMode() {
			return dev.ErrNotInDevMode
		}

		return repo.VerifyRefFromEntry(cmd.Context(), args[0], o.fromEntry, verifyopts.WithOverrideRefName(o.remoteRefName))
	}

	if o.latestOnly {
		opts = append(opts, verifyopts.WithLatestOnly())
	}
	return repo.VerifyRef(cmd.Context(), args[0], opts...)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "verify-ref",
		Short:             "Tools for verifying gittuf policies",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
