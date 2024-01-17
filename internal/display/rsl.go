// SPDX-License-Identifier: Apache-2.0

package display

import (
	"fmt"

	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/plumbing"
)

// PrepareRSLLogOutput takes the RSL, and returns a string representation of it,
// with annotations attached to entries
/* Output format:
entry <entryID> (skipped)

  Ref:    <refName>
  Target: <targetID>

    Annotation ID: <annotationID>
    Skip:          <yes/no>
    Message:
      <message>

    Annotation ID: <annotationID>
    Skip:          <yes/no>
    Message:
      <message>
*/
func PrepareRSLLogOutput(entries []*rsl.ReferenceEntry, annotationMap map[plumbing.Hash][]*rsl.AnnotationEntry) string {
	log := ""

	for _, entry := range entries {
		log += fmt.Sprintf("entry %v", entry.ID)

		skipped := false
		if annotations, ok := annotationMap[entry.ID]; ok {
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

		if annotations, ok := annotationMap[entry.ID]; ok {
			for _, annotation := range annotations {
				log += "\n"
				log += fmt.Sprintf("\n    Annotation ID: %s", annotation.ID.String())
				if annotation.Skip {
					log += "\n    Skip:          yes"
				} else {
					log += "\n    Skip:          no"
				}
				log += fmt.Sprintf("\n    Message:\n      %s", annotation.Message)
			}
		}

		log += "\n\n"
	}

	return log[:len(log)-1]
}
