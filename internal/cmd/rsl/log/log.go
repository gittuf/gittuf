// SPDX-License-Identifier: Apache-2.0

package log

import (
	"fmt"
	"os"

	"github.com/gittuf/gittuf/internal/display"
	"github.com/gittuf/gittuf/internal/repository"
	"github.com/spf13/cobra"
)

type options struct{}

func (o *options) AddFlags(_ *cobra.Command) {}

func (o *options) Run(_ *cobra.Command, _ []string) error {
	repo, err := repository.LoadRepository()
	if err != nil {
		return err
	}

	entries, annotationMap, err := repository.GetRSLEntryLog(repo)
	if err != nil {
		return err
	}

	output := display.PrepareRSLLogOutput(entries, annotationMap)
	if err := display.Display(os.Stdout, os.Stderr, []byte(output), true); err != nil {
		return fmt.Errorf("unable to display RSL entries: %s", err.Error())
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "log",
		Short:             "Displays the Reference State Log",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
