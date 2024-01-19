// SPDX-License-Identifier: Apache-2.0

package clone

import (
	"os"

	"github.com/gittuf/gittuf/internal/repository"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/spf13/cobra"
)

type options struct {
	branch           string
	expectedRootKeys []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVarP(
		&o.branch,
		"branch",
		"b",
		"",
		"specify branch to check out",
	)
	cmd.Flags().StringSliceVarP(
		&o.expectedRootKeys,
		"expectedRootKeys",
		"e",
		[]string{},
		"specify expected root keys in cloned repo",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	var dir string
	if len(args) > 1 {
		dir = args[1]
	}
	expectedRootKeys := []*tuf.Key{}

	for _, keyPath := range o.expectedRootKeys {
		keyBytes, err := os.ReadFile(keyPath)
		if err != nil {
			return err
		}

		key, err := tuf.LoadKeyFromBytes(keyBytes)
		if err != nil {
			return err
		}

		expectedRootKeys = append(expectedRootKeys, key)
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
