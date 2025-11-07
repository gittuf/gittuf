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
	latestOnly       bool
	fromEntry        string
	remoteRefName    string
	granularVSAsPath string
	metaVSAPath      string
	vsaSigner        string
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

	cmd.Flags().StringVar(
		&o.granularVSAsPath,
		"write-all-vsas",
		"",
		"path to write source verification summary attestations (one per policy) as an in-toto attestation bundle",
	)

	cmd.Flags().StringVar(
		&o.metaVSAPath,
		"write-unified-vsa",
		"",
		"path to write a single source verification summary attestation",
	)

	cmd.Flags().StringVar(
		&o.vsaSigner,
		"sign-source-attestation",
		"",
		"signing key or identity for one or more source provenance or verification attestations",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	if o.fromEntry != "" {
		if !dev.InDevMode() {
			return dev.ErrNotInDevMode
		}

		return repo.VerifyRefFromEntry(cmd.Context(), args[0], o.fromEntry, verifyopts.WithOverrideRefName(o.remoteRefName))
	}

	opts := []verifyopts.Option{verifyopts.WithOverrideRefName(o.remoteRefName)}
	if o.latestOnly {
		opts = append(opts, verifyopts.WithLatestOnly())
	}

	if o.granularVSAsPath != "" {
		opts = append(opts, verifyopts.WithGranularVSAsPath(o.granularVSAsPath))
	}
	if o.metaVSAPath != "" {
		opts = append(opts, verifyopts.WithMetaVSAPath(o.metaVSAPath))
	}

	if o.vsaSigner != "" {
		signer, err := gittuf.LoadSigner(repo, o.vsaSigner)
		if err != nil {
			return err
		}

		opts = append(opts, verifyopts.WithVSASigner(signer))
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
