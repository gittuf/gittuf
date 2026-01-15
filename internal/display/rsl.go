// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/pkg/gitinterface"
)

// RSLLog implements the display function for `gittuf rsl log`.
func RSLLog(repo *gitinterface.Repository, writer io.WriteCloser) error {
	defer writer.Close() //nolint:errcheck

	annotationsMap := make(map[string][]*rsl.AnnotationEntry)

	iteratorEntry, err := rsl.GetLatestEntry(repo)
	if err != nil {
		return err
	}

	for {
		hasParent := true // assume an entry has a parent
		parentEntry, err := rsl.GetParentForEntry(repo, iteratorEntry)
		if err != nil {
			if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
				return err
			}

			// only reachable when err is ErrRSLEntryNotFound
			// Now we know the iteratorEntry does not have a parent
			hasParent = false
		}

		switch iteratorEntry := iteratorEntry.(type) {
		case *rsl.ReferenceEntry:
			slog.Debug(fmt.Sprintf("Writing reference entry '%s'...", iteratorEntry.ID.String()))
			if err := writeRSLReferenceEntry(writer, iteratorEntry, annotationsMap[iteratorEntry.ID.String()], hasParent); err != nil {
				// We return nil here to avoid noisy output when the writer is
				// unexpectedly closed, such as by killing the pager
				return nil
			}
		case *rsl.AnnotationEntry:
			slog.Debug(fmt.Sprintf("Tracking annotation entry '%s'...", iteratorEntry.ID.String()))
			for _, targetID := range iteratorEntry.RSLEntryIDs {
				targetIDString := targetID.String()

				if _, has := annotationsMap[targetIDString]; !has {
					annotationsMap[targetIDString] = []*rsl.AnnotationEntry{}
				}

				annotationsMap[targetIDString] = append(annotationsMap[targetIDString], iteratorEntry)
			}

		case *rsl.PropagationEntry:
			slog.Debug(fmt.Sprintf("Writing propagation entry '%s'...", iteratorEntry.ID.String()))
			if err := writeRSLPropagationEntry(writer, iteratorEntry, hasParent); err != nil {
				// We return nil here to avoid noisy output when
				// the writer is unexpectedly closed, such as by
				// killing the pager
				return nil
			}
		}

		if !hasParent {
			// We're done
			return nil
		}

		iteratorEntry = parentEntry
	}
}

// writeRSLReferenceEntry prepares the output for the given entry and its
// annotations. It then writes the output to the provided writer. If hasParent
// is false, then the prepared output for the entry has a single trailing
// newline. Otherwise, an additional newline is added to separate entries from
// one another.
func writeRSLReferenceEntry(writer io.WriteCloser, entry *rsl.ReferenceEntry, annotations []*rsl.AnnotationEntry, hasParent bool) error {
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

	text := colorer(fmt.Sprintf("entry %s", entry.ID.String()), yellow)

	for _, annotation := range annotations {
		if annotation.Skip {
			text += fmt.Sprintf(" %s", colorer("(skipped)", red))
			break
		}
	}

	text += "\n"

	text += fmt.Sprintf("\n  Ref:    %s", entry.RefName)
	text += fmt.Sprintf("\n  Target: %s", entry.TargetID.String())
	if entry.Number != 0 {
		text += fmt.Sprintf("\n  Number: %d", entry.Number)
	}

	for _, annotation := range annotations {
		text += "\n\n"
		text += colorer(fmt.Sprintf("    Annotation ID: %s", annotation.ID.String()), green)
		text += "\n"
		if annotation.Skip {
			text += colorer("    Skip:          yes", red)
		} else {
			text += "    Skip:          no"
		}
		if annotation.Number != 0 {
			text += fmt.Sprintf("\n    Number:        %d", annotation.Number)
		}
		text += fmt.Sprintf("\n    Message:\n      %s", strings.TrimSpace(annotation.Message))
	}

	text += "\n" // single trailing newline by default
	if hasParent {
		text += "\n" // extra newline for all intermediate (i.e., not last) entries
	}

	_, err := writer.Write([]byte(text))
	return err
}

func writeRSLPropagationEntry(writer io.WriteCloser, entry *rsl.PropagationEntry, hasParent bool) error {
	/* Output format:
	   propagation entry <entryID>
	     Ref:           <refName>
	     Target:        <targetID>
		 UpstreamRepo:  <upstreamRepoLocation>
		 UpstreamEntry: <upstreamEntryID>
	     Number:        <number>
	*/

	text := colorer(fmt.Sprintf("propagation entry %s", entry.ID.String()), yellow)
	text += "\n"

	text += fmt.Sprintf("\n  Ref:           %s", entry.RefName)
	text += fmt.Sprintf("\n  Target:        %s", entry.TargetID.String())
	text += fmt.Sprintf("\n  UpstreamRepo:  %s", entry.UpstreamRepository)
	text += fmt.Sprintf("\n  UpstreamEntry: %s", entry.UpstreamEntryID.String())
	if entry.Number != 0 {
		text += fmt.Sprintf("\n  Number:        %d", entry.Number)
	}

	text += "\n" // single trailing newline by default
	if hasParent {
		text += "\n" // extra newline for all intermediate (i.e., not last) entries
	}

	_, err := writer.Write([]byte(text))
	return err
}
