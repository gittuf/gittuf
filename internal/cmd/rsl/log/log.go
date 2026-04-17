// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package log //nolint:revive

import (
	"os"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/display"
	"github.com/spf13/cobra"
)

type options struct {
	refs []string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().StringArrayVar(
		&o.refs,
		"ref",
		nil,
		"only display RSL entries for the specified references",
	)
}

func (o *options) Run(_ *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	return display.RSLLog(repo.GetGitRepository(), display.NewDisplayWriter(os.Stdout), display.WithReferences(o.refs))
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "log",
		Short:             "Display the repository's Reference State Log",
		Long:              "The 'log' command displays the repository's RSL. It is used to view the history of reference state changes and inspect prior entries in the RSL.",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
