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
		"set of initial root of trust keys for the repository (supported values: paths to SSH keys, GPG key fingerprints, Sigstore/Fulcio identities)",
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
		Long:              `The 'clone' command clones a gittuf-enabled Git repository along with its associated gittuf metadata. This command can also ensure the repository's trust root is established correctly by using specified root keys, optionally supplied using the --root-key flag. You may also specify a particular branch to check out with --branch and choose whether to create a bare repository using --bare.`,
		Args:              cobra.MinimumNArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
