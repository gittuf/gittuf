// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package clone

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	branch           string
	expectedRootKeys common.PublicKeys
	bare             bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.branch,
		"branch",
		"b",
		"",
		"specify branch to check out",
	)

	cmd.Flags().Var(
		&o.expectedRootKeys,
		"root-key",
		"set of initial root of trust keys for the repository (each a path to an SSH public key, \"gpg:<fingerprint>\" for GPG, or \"fulcio:<identity>::<issuer>\" for Sigstore)",
	)

	cmd.Flags().BoolVar(
		&o.bare,
		"bare",
		false,
		"make a bare Git repository",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	var dir string
	if len(args) > 1 {
		dir = args[1]
	}

	expectedRootKeys := make([]tuf.Principal, len(o.expectedRootKeys))

	for index, keyPath := range o.expectedRootKeys {
		key, err := gittuf.LoadPublicKey(keyPath)
		if err != nil {
			return err
		}

		expectedRootKeys[index] = key
	}

	_, err := gittuf.Clone(cmd.Context(), args[0], dir, o.branch, expectedRootKeys, o.bare)
	return err
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "clone",
		Short:             "Clone repository and its gittuf references",
		Long:              "The 'clone' command clones a gittuf-enabled Git repository along with its associated gittuf metadata. It is used to obtain a repository and verify its RSL and policy against the provided root of trust keys.",
		Args:              cobra.RangeArgs(1, 2),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
