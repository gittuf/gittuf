// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifyref

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	verifyopts "github.com/gittuf/gittuf/experimental/gittuf/options/verify"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	latestOnly       bool
	fromEntry        string
	expectedRootKeys common.PublicKeys
	remoteRefName    string
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

	cmd.Flags().Var(
		&o.expectedRootKeys,
		"root-key",
		"set of initial root of trust keys for the repository (supported values: paths to SSH keys, GPG key fingerprints, Sigstore/Fulcio identities)",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	var opts []verifyopts.Option

	if len(o.expectedRootKeys) > 0 {
		expectedRootKeys := make([]tuf.Principal, len(o.expectedRootKeys))

		for index, keyPath := range o.expectedRootKeys {
			key, err := gittuf.LoadPublicKey(keyPath)
			if err != nil {
				return err
			}

			expectedRootKeys[index] = key
		}

		opts = append(opts, verifyopts.WithExpectedRootKeys(expectedRootKeys))
	}

	if o.fromEntry != "" {
		if !dev.InDevMode() {
			return dev.ErrNotInDevMode
		}

		opts = append(opts, verifyopts.WithOverrideRefName(o.remoteRefName))
		return repo.VerifyRefFromEntry(cmd.Context(), args[0], o.fromEntry, opts...)
	}

	opts = append(opts, verifyopts.WithOverrideRefName(o.remoteRefName))
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
