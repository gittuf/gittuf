// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"errors"
	"fmt"
	"io"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
)

// PrintRSLEntryLog prints all rsl entries to the console, first printing all
// reference entries, and then all annotation entries with their corresponding
// references.
func PrintRSLEntryLog(repo *gitinterface.Repository, bufferedWriter io.WriteCloser) error {
	defer bufferedWriter.Close() //nolint:errcheck

	allReferenceEntries := []*rsl.ReferenceEntry{}
	emptyAnnotationMap := make(map[string][]*rsl.AnnotationEntry)
	annotationMap := make(map[string][]*rsl.AnnotationEntry)

	iteratorEntry, err := rsl.GetLatestEntry(repo)
	if err != nil {
		return err
	}

	// Display header and all reference entries
	if err := PrintHeader(bufferedWriter, "Reference Entries"); err != nil {
		return nil
	}

	for {
		switch iteratorEntry := iteratorEntry.(type) {
		case *rsl.ReferenceEntry:
			allReferenceEntries = append(allReferenceEntries, iteratorEntry)

			// emptyAnnotationMap forces reference entries to be printed
			// without the corresponding annotations
			if err := BufferedLogToConsole([]*rsl.ReferenceEntry{iteratorEntry}, emptyAnnotationMap, bufferedWriter); err != nil {
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

	// Display header and all annotation entries
	if len(annotationMap) != 0 {
		if err := PrintHeader(bufferedWriter, "Annotation Entries"); err != nil {
			return nil
		}
	}

	for _, entry := range allReferenceEntries {
		targetID := entry.GetID().String()
		if _, exists := annotationMap[targetID]; exists {
			if err := BufferedLogToConsole([]*rsl.ReferenceEntry{entry}, annotationMap, bufferedWriter); err != nil {
				return nil
			}
		}
	}

	return nil
}

func BufferedLogToConsole(entries []*rsl.ReferenceEntry, annotationMap map[string][]*rsl.AnnotationEntry, bufferedWriter io.WriteCloser) error {
	formatedOutput := PrepareRSLLogOutput(entries, annotationMap)

	_, err := bufferedWriter.Write([]byte(formatedOutput))
	if err != nil {
		return err
	}

	return nil
}

func PrintHeader(bufferedWriter io.WriteCloser, title string) error {
	header := fmt.Sprintf(
		"---------------------------------------------------\n%s\n---------------------------------------------------\n\n",
		title,
	)

	_, err := bufferedWriter.Write([]byte(header))
	return err
}

// PrepareRSLLogOutput takes the RSL, and returns a string representation of it,
// with annotations attached to entries
/* Output format:
entry <entryID> (skipped)

  Ref:    <refName>
  Target: <targetID>
  Number: <number>

    Annotation ID: <annotationID>
    Skip:          <yes/no>
    Number:        <number>
    Message:
      <message>

    Annotation ID: <annotationID>
    Skip:          <yes/no>
    Number:        <number>
    Message:
      <message>
*/
func PrepareRSLLogOutput(entries []*rsl.ReferenceEntry, annotationMap map[string][]*rsl.AnnotationEntry) string {
	log := ""

	for _, entry := range entries {
		log += fmt.Sprintf("entry %s", entry.ID.String())

		skipped := false
		if annotations, ok := annotationMap[entry.ID.String()]; ok {
			for _, annotation := range annotations {
				if annotation.Skip {
					skipped = true
					break
				}
			}
		}

		if skipped {
			log += " (skipped)"
		}
		log += "\n"

		log += fmt.Sprintf("\n  Ref:    %s", entry.RefName)
		log += fmt.Sprintf("\n  Target: %s", entry.TargetID.String())
		if entry.Number != 0 {
			log += fmt.Sprintf("\n  Number: %d", entry.Number)
		}

		if annotations, ok := annotationMap[entry.ID.String()]; ok {
			for _, annotation := range annotations {
				log += "\n"
				log += fmt.Sprintf("\n    Annotation ID: %s", annotation.ID.String())
				if annotation.Skip {
					log += "\n    Skip:          yes"
				} else {
					log += "\n    Skip:          no"
				}
				if annotation.Number != 0 {
					log += fmt.Sprintf("\n    Number:        %d", annotation.Number)
				}
				log += fmt.Sprintf("\n    Message:\n      %s", annotation.Message)
			}
		}

		log += "\n\n"
	}

	return log
}
