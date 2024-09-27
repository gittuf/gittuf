// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package clone

import (
	"github.com/gittuf/gittuf/internal/cmd/common"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	branch           string
	expectedRootKeys common.PublicKeys
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
		"set of initial root of trust keys for the repository (supported values: paths to SSH keys, GPG key fingerprints, Sigstore/Fulcio identities)",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	var dir string
	if len(args) > 1 {
		dir = args[1]
	}

	expectedRootKeys := make([]*tuf.Key, len(o.expectedRootKeys))

	for index, keyPath := range o.expectedRootKeys {
		key, err := common.LoadPublicKey(keyPath)
		if err != nil {
			return err
		}

		expectedRootKeys[index] = key
	}

	_, err := repository.Clone(cmd.Context(), args[0], dir, o.branch, expectedRootKeys)
	return err
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "clone",
		Short:             "Clone repository and its gittuf references",
		Args:              cobra.MinimumNArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
