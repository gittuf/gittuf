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
		"stage policy changes locally without pushing to a remote repository",
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
		Long:  "The 'stage' command stages local policy changes and records them in the RSL. It optionally pushes the staged changes to a remote repository. It is used to prepare policy updates so they can be reviewed and signed by other users if needed.",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
