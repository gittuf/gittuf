// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"os"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/display"
	"github.com/gittuf/gittuf/internal/rsl"
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

	output := os.Stdout
	if o.filePath != "" {
		output, err = os.Create(o.filePath)
		if err != nil {
			return err
		}
		o.page = false // override page since we're not writing to stdout
	}

	bufferedWriter := display.NewDisplayWriter(output, o.page)
	entries, annotationMap, err := gittuf.GetRSLEntryLog(repo, false)

	for i := 0; i < len(entries); i++ {
		arrayOfCurrentEntry := []*rsl.ReferenceEntry{entries[i]}
		formatedOutput := display.PrepareRSLLogOutput(arrayOfCurrentEntry, annotationMap)
		_, err = bufferedWriter.Write([]byte(formatedOutput))
		if err != nil {
			return err
		}
	}

	err = bufferedWriter.Close()
	if err != nil {
		return err
	}

	return nil
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
