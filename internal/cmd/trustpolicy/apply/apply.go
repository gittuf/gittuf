// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
	localOnly bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&o.localOnly,
		"local-only",
		false,
		"apply policy changes locally without pushing to a remote repository",
	)
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	remoteName := ""
	if len(args) > 0 {
		remoteName = args[0]
	}

	return repo.ApplyPolicy(cmd.Context(), remoteName, o.localOnly, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "apply",
		Short: "Validate and apply changes from policy-staging to policy",
		Long:  "The 'apply' command validates and applies changes from the policy-staging area to the repository's policy. It is used to make staged policy updates effective and records the change in the RSL. Pass '--local-only' to apply without pushing upstream. Otherwise, supply the remote name as the first positional argument.",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
