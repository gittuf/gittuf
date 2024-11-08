// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"os"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/display"
	"github.com/spf13/cobra"
)

type options struct {
	page     bool
	filePath string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVar(
		&o.page,
		"page",
		true,
		"page log using system's default PAGER, only enabled if displaying to stdout",
	)

	cmd.Flags().StringVar(
		&o.filePath,
		"file",
		"",
		"write log to file at specified path",
	)
}

func (o *options) Run(_ *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository()
	if err != nil {
		return err
	}

	entries, annotationMap, err := gittuf.GetRSLEntryLog(repo)
	if err != nil {
		return err
	}

	output := os.Stdout
	if o.filePath != "" {
		output, err = os.Create(o.filePath)
		if err != nil {
			return err
		}
		o.page = false // override page since we're not writing to stdout
	}

	outputContents := display.PrepareRSLLogOutput(entries, annotationMap)
	writer := display.NewDisplayWriter(output, o.page)

	_, err = writer.Write([]byte(outputContents))
	return err
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:               "log",
		Short:             "Display the Reference State Log",
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
