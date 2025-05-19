// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhooks

import (
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
	force bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(
		&o.force,
		"force",
		"f",
		false,
		"overwrite hooks, if they already exist",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	err = repo.UpdateGitHook(gittuf.HookPrePush, prePushScript, o.force)
	var hookErr *gittuf.ErrHookExists
	if errors.As(err, &hookErr) {
		fmt.Fprintf(
			cmd.ErrOrStderr(),
			"'%s' already exists. Use --force flag or merge existing hook and the following script manually:\n\n%s\n",
			string(hookErr.HookType),
			prePushScript,
		)
	}
	return err
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "add-hooks",
		Short:             "Add git hooks that automatically create and sync RSL",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
