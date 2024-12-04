// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"fmt"
	"io"

	"github.com/gittuf/gittuf/internal/rsl"
)

type FunctionHolder struct {
	DisplayLog    func([]*rsl.ReferenceEntry, map[string][]*rsl.AnnotationEntry, io.WriteCloser) error
	DisplayHeader func(io.WriteCloser, string) error
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
