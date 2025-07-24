// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"os"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/display"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(_ *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	return display.RSLLog(repo.GetGitRepository(), display.NewDisplayWriter(os.Stdout))
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "log",
		Short: "Display the repository's Reference State Log",
		Long: `The 'log' command shows the entries in the repository's Repository Signing Log (RSL).
The RSL tracks significant security-related events and changes in the repository.
This log helps with auditing and reviewing the repository's trusted state over time.`,

		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
