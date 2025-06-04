// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rslrecordat

import (
	"fmt"
	"os"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

type options struct {
	targetID       string
	signingKeyPath string
	dstRef         string
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
}

func (o *options) Run(_ *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	signingKeyBytes, err := os.ReadFile(o.signingKeyPath)
	if err != nil {
		return err
	}

	return repo.RecordRSLEntryForReferenceAtTarget(args[0], o.targetID, signingKeyBytes, rslopts.WithOverrideRefName(o.dstRef))
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "rsl-record",
		Short: fmt.Sprintf("Record explicit state of a Git reference in the RSL, signed with specified key (developer mode only, set %s=1)", dev.DevModeKey),
		Long:  fmt.Sprintf(`The 'rsl-record' command records the explicit state of a Git reference into the RSL. This command is intended for developer and testing workflows where manual creation of entries in the RSL for a specific Git reference (e.g., branch or tag) at a given commit (target ID) is required. You can optionally specify a destination reference name using --dst-ref to override the default. It requires developer mode to be enabled by setting the environment variable %s=1.`, dev.DevModeKey),
		Args:  cobra.ExactArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
