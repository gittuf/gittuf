// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"errors"
	"io"
	"os"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/display"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/spf13/cobra"
)

// PrintRSLEntryLog prints all rsl entries to the console, first printing all
// reference entries, and then all annotation entries with their corresponding
// references.
func PrintRSLEntryLog(repo *gitinterface.Repository, bufferedWriter io.WriteCloser, display display.FunctionHolder) error {
	defer bufferedWriter.Close() //nolint:errcheck

	allReferenceEntries := []*rsl.ReferenceEntry{}
	emptyAnnotationMap := make(map[string][]*rsl.AnnotationEntry)
	annotationMap := make(map[string][]*rsl.AnnotationEntry)

	iteratorEntry, err := rsl.GetLatestEntry(repo)
	if err != nil {
		return err
	}

	if err := display.DisplayHeader(bufferedWriter, "Reference Entries"); err != nil {
		return nil
	}

	// Display all reference entries
	for {
		switch iteratorEntry := iteratorEntry.(type) {
		case *rsl.ReferenceEntry:
			allReferenceEntries = append(allReferenceEntries, iteratorEntry)
			if err := display.DisplayLog([]*rsl.ReferenceEntry{iteratorEntry}, emptyAnnotationMap, bufferedWriter); err != nil {
				return nil
			}
		case *rsl.AnnotationEntry:
			for _, targetID := range iteratorEntry.RSLEntryIDs {
				if _, has := annotationMap[targetID.String()]; !has {
					annotationMap[targetID.String()] = []*rsl.AnnotationEntry{}
				}

				annotationMap[targetID.String()] = append(annotationMap[targetID.String()], iteratorEntry)
			}
		}

		parentEntry, err := rsl.GetParentForEntry(repo, iteratorEntry)
		if err != nil {
			if errors.Is(err, rsl.ErrRSLEntryNotFound) {
				break
			}

			return err
		}

		iteratorEntry = parentEntry
	}

	if len(annotationMap) != 0 {
		if err := display.DisplayHeader(bufferedWriter, "Annotation Entries"); err != nil {
			return nil
		}
	}

	// Display all annotation entries
	for _, entry := range allReferenceEntries {
		targetID := entry.GetID().String()
		if _, exists := annotationMap[targetID]; exists {
			if err := display.DisplayLog([]*rsl.ReferenceEntry{entry}, annotationMap, bufferedWriter); err != nil {
				return nil
			}
		}
	}

	return nil
}

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
	d := display.FunctionHolder{
		DisplayLog:    display.BufferedLogToConsole,
		DisplayHeader: display.PrintHeader,
	}

	err = PrintRSLEntryLog(repo.GetGitRepository(), bufferedWriter, d)
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
