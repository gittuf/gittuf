// SPDX-License-Identifier: Apache-2.0

package check

import (
	"fmt"

	repository "github.com/gittuf/gittuf/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
}

func (o *options) Run(cmd *cobra.Command, args []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	hasUpdates, hasDiverged, err := repo.CheckRemoteRSLForUpdates(cmd.Context(), args[0])
	if err != nil {
		return err
	}

	if hasUpdates {
		fmt.Printf("RSL at remote %s has updates", args[0])
		if hasDiverged {
			fmt.Printf(" and has diverged from local RSL")
		}
	} else {
		fmt.Printf("RSL at remote %s has no updates", args[0])
	}

	fmt.Println() // Trailing newline

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "check <remote>",
		Short:             "Check remote RSL for updates, for development use only",
		Args:              cobra.ExactArgs(1),
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}

	return cmd
}
