// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"errors"
	"fmt"
	"io"
	"log/slog"
	"maps"
	"slices"
	"strings"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/gittuf/gittuf/pkg/rsl"
)

type options struct {
	refs *set.Set[string]
}

type Option func(*options)

func WithReferences(refs []string) Option {
	return func(o *options) {
		o.refs = set.NewSetFromItems(refs...)
	}
}

// RSLLog implements the display function for `gittuf rsl log`.
func RSLLog(repo gitstore.Storer, writer io.WriteCloser, opts ...Option) error {
	defer writer.Close() //nolint:errcheck

	options := &options{refs: set.NewSet[string]()}
	for _, fn := range opts {
		fn(options)
	}

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
			if options.refs.Len() != 0 && !options.refs.Has(iteratorEntry.RefName) {
				// Skip this entry if it's not for the specified ref. Note that
				// we still want to track annotation entries for this entry (if
				// there are any) since they may apply to other entries that we
				// do want to display.
				slog.Debug(fmt.Sprintf("Skipping reference entry '%s' since it is for ref '%s'...", iteratorEntry.ID.String(), iteratorEntry.RefName))
				break
			}

			slog.Debug(fmt.Sprintf("Writing reference entry '%s'...", iteratorEntry.ID.String()))
			if err := writeRSLReferenceEntry(writer, iteratorEntry, annotationsMap[iteratorEntry.ID.String()], hasParent); err != nil {
				// We return nil here to avoid noisy output when the writer is
				// unexpectedly closed, such as by killing the pager
				return nil
			}
		case *rsl.BulkReferenceEntry:
			// The whole entry renders when any update matches the filter,
			// because the entry is one signed act.
			if options.refs.Len() != 0 && !bulkTouchesAnyRef(iteratorEntry, options.refs) {
				slog.Debug(fmt.Sprintf("Skipping bulk reference entry '%s' since none of its refs were requested...", iteratorEntry.ID.String()))
				break
			}

			slog.Debug(fmt.Sprintf("Writing bulk reference entry '%s'...", iteratorEntry.ID.String()))
			if err := writeRSLBulkReferenceEntry(writer, iteratorEntry, annotationsMap[iteratorEntry.ID.String()], hasParent); err != nil {
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
			if options.refs.Len() != 0 && !options.refs.Has(iteratorEntry.RefName) {
				// Skip this entry if it's not for the specified ref. Note that
				// we still want to track annotation entries for this entry (if
				// there are any) since they may apply to other entries that we
				// do want to display.
				slog.Debug(fmt.Sprintf("Skipping propagation entry '%s' since it is for ref '%s'...", iteratorEntry.ID.String(), iteratorEntry.RefName))
				break
			}

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
	     Custom Fields:
	       <key>: <value>

	       Annotation ID: <annotationID>
	       Skip:          <yes/no>
	       Number:        <number>
	       Custom Fields:
	         <key>: <value>
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
	text = appendCustomFields(text, entry.CustomFields, "  ")

	text += formatAnnotations(annotations, entry.ID.String())

	text += "\n" // single trailing newline by default
	if hasParent {
		text += "\n" // extra newline for all intermediate (i.e., not last) entries
	}

	_, err := writer.Write([]byte(text))
	return err
}

// bulkTouchesAnyRef reports whether any of the entry's updates is for one of
// the requested refs.
func bulkTouchesAnyRef(entry *rsl.BulkReferenceEntry, refs *set.Set[string]) bool {
	for _, update := range entry.Updates {
		if refs.Has(update.RefName) {
			return true
		}
	}
	return false
}

// writeRSLBulkReferenceEntry renders a bulk entry as one block listing every
// update, followed by its annotations. A qualified annotation lists the refs
// it applies to.
func writeRSLBulkReferenceEntry(writer io.WriteCloser, entry *rsl.BulkReferenceEntry, annotations []*rsl.AnnotationEntry, hasParent bool) error {
	/* Output format:
	   bulk entry <entryID> (skipped)

	     Ref:    <refName>
	     Target: <targetID>

	     Ref:    <refName>
	     Target: <targetID>

	     Number: <number>
	     Custom Fields:
	       <key>: <value>

	       Annotation ID: <annotationID>
	       Skip:          <yes/no>
	       Refs:          <ref>, <ref>
	       Number:        <number>
	       Message:
	         <message>
	*/

	text := colorer(fmt.Sprintf("bulk entry %s", entry.ID.String()), yellow)

	for _, annotation := range annotations {
		if annotation.Skip && len(annotation.Refs[entry.ID.String()]) == 0 {
			text += fmt.Sprintf(" %s", colorer("(skipped)", red))
			break
		}
	}
	text += "\n"

	for i, update := range entry.Updates {
		if i != 0 {
			text += "\n"
		}
		text += fmt.Sprintf("\n  Ref:    %s", update.RefName)
		text += fmt.Sprintf("\n  Target: %s", update.TargetID.String())
	}
	if entry.Number != 0 {
		text += "\n"
		text += fmt.Sprintf("\n  Number: %d", entry.Number)
	}
	text = appendCustomFields(text, entry.CustomFields, "  ")

	text += formatAnnotations(annotations, entry.ID.String())

	text += "\n"
	if hasParent {
		text += "\n"
	}

	_, err := writer.Write([]byte(text))
	return err
}

// formatAnnotations renders the annotation blocks shared by reference and
// bulk entries. entryID selects which qualifiers to show.
func formatAnnotations(annotations []*rsl.AnnotationEntry, entryID string) string {
	text := ""
	for _, annotation := range annotations {
		text += "\n\n"
		text += colorer(fmt.Sprintf("    Annotation ID: %s", annotation.ID.String()), green)
		text += "\n"
		if annotation.Skip {
			text += colorer("    Skip:          yes", red)
		} else {
			text += "    Skip:          no"
		}
		if refs := annotation.Refs[entryID]; len(refs) != 0 {
			text += fmt.Sprintf("\n    Refs:          %s", strings.Join(refs, ", "))
		}
		if annotation.Number != 0 {
			text += fmt.Sprintf("\n    Number:        %d", annotation.Number)
		}
		text = appendCustomFields(text, annotation.CustomFields, "    ")
		text += fmt.Sprintf("\n    Message:\n      %s", strings.TrimSpace(annotation.Message))
	}
	return text
}

func writeRSLPropagationEntry(writer io.WriteCloser, entry *rsl.PropagationEntry, hasParent bool) error {
	/* Output format:
	   propagation entry <entryID>
	     Ref:           <refName>
	     Target:        <targetID>
		 UpstreamRepo:  <upstreamRepoLocation>
		 UpstreamEntry: <upstreamEntryID>
	     Number:        <number>
	     Custom Fields:
	       <key>: <value>
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
	text = appendCustomFields(text, entry.CustomFields, "  ")

	text += "\n" // single trailing newline by default
	if hasParent {
		text += "\n" // extra newline for all intermediate (i.e., not last) entries
	}

	_, err := writer.Write([]byte(text))
	return err
}

func appendCustomFields(text string, fields rsl.CustomFields, indent string) string {
	if len(fields) == 0 {
		return text
	}

	text += fmt.Sprintf("\n%sCustom Fields:", indent)
	for _, key := range slices.Sorted(maps.Keys(fields)) {
		text += fmt.Sprintf("\n%s  %s: %s", indent, key, fields[key])
	}
	return text
}
