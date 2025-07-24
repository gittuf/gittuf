// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package propagate

import (
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	return repo.PropagateChangesFromUpstreamRepositories(cmd.Context(), true)
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "propagate",
		Short: fmt.Sprintf("Propagate contents of remote repositories into local repository (developer mode only, set %s=1)", dev.DevModeKey),
		Long: fmt.Sprintf(`The 'propagate' command copies RSL entries from upstream repositories into the local repository.
This command is only available in developer mode (enable with %s=1).
It helps simulate or test how upstream RSL changes affect the local repository.`, dev.DevModeKey),

		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
