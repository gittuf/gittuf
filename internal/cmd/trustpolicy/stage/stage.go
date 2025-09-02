// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package stage

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
		"indicate that the policy must be committed into the RSL locally",
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

	return repo.StagePolicy(cmd.Context(), remoteName, o.localOnly, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "stage",
		Short: "Stage and push local policy-staging changes to remote repository",
		Long: `Stage records local policy changes into 'policy-staging' for validation
before they are applied to the active policy. By default, staged changes are
also pushed to a remote repository. You may provide a remote name to target a
specific remote, or use '--local-only' to stage changes only in the local
repository.`,

		RunE: o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
