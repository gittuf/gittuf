// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifymergeable

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
	baseBranch    string
	featureBranch string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringVar(
		&o.baseBranch,
		"base-branch",
		"",
		"base branch for proposed merge",
	)
	cmd.MarkFlagRequired("base-branch") //nolint:errcheck

	cmd.Flags().StringVar(
		&o.featureBranch,
		"feature-branch",
		"",
		"feature branch for proposed merge",
	)
	cmd.MarkFlagRequired("feature-branch") //nolint:errcheck
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	_, err = repo.VerifyMergeable(cmd.Context(), o.baseBranch, o.featureBranch)
	return err
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "verify-mergeable",
		Short:             "Tools for verifying mergeability using gittuf policies",
		Args:              cobra.ExactArgs(0),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
