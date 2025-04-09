// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifymergeable

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/experimental/gittuf/options/verifymergeable"
	"github.com/spf13/cobra"
)

type options struct {
	baseBranch    string
	featureBranch string
	bypassRSL     bool
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

	cmd.Flags().BoolVar(
		&o.bypassRSL,
		"bypass-RSL",
		false,
		"bypass RSL when identifying current state of feature ref",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	opts := []verifymergeable.Option{}
	if o.bypassRSL {
		opts = append(opts, verifymergeable.WithBypassRSLForFeatureRef())
	}

	_, err = repo.VerifyMergeable(cmd.Context(), o.baseBranch, o.featureBranch, opts...)
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
