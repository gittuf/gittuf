// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package propagate

import (
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	return repo.PropagateChangesFromUpstreamRepositories(cmd.Context(), true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "propagate",
		Short:             `Propagate contents of remote repositories into local repository`,
		Long:              "The 'propagate' command propagates RSL contents from remote repositories defined in propagation directives into the local repository. It is used to record a snapshot of the propagated RSL and repository contents locally.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
