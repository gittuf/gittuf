// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package sync

import (
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
	overwriteLocalRefs bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&o.overwriteLocalRefs,
		"overwrite",
		false,
		"overwrite local references with upstream changes",
	)
}

func (o *options) Run(_ *cobra.Command, args []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	remoteName := "origin"
	if len(args) > 0 {
		remoteName = args[0]
	}

	divergedRefs, err := repo.Sync(remoteName, o.overwriteLocalRefs)
	switch {
	case errors.Is(err, gittuf.ErrDivergedRefs):
		fmt.Println("References have diverged:")
		for _, refName := range divergedRefs {
			fmt.Println(refName)
		}
		fmt.Println("To apply upstream changes locally, rerun the command with --overwrite. WARNING: this operation may result in the loss of local work!")
		return nil
	default:
		return err
	}
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "sync [remoteName]",
		Short: "Synchronize local references with remote references based on RSL",
		Args:  cobra.MaximumNArgs(1),
		RunE:  o.Run,
	}
	o.AddFlags(cmd)

	return cmd
}
