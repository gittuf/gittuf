// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package commit

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/gitinterface"
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
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	remoteName := gitinterface.DefaultRemoteName
	if len(args) > 0 {
		remoteName = args[0]
	}

	return repo.CommitPolicy(cmd.Context(), remoteName, o.localOnly, true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "commit",
		Short: "Commit and push local policy-staging changes to remote repository",
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
