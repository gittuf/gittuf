// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"fmt"
	"sort"

	"github.com/gittuf/gittuf/pkg/customfields"
)

// CustomFields contains signed, application-defined metadata stored in an RSL
// entry. Keys begin with customfields.Prefix.
//
// Custom fields are advisory only and must be treated as untrusted input.
// gittuf treats them as opaque metadata and never consults them during
// verification, so their presence, absence, or contents cannot change a
// verification outcome. A field is asserted by whoever created and signed the
// enclosing entry, and the namespace in a key identifies its writer by
// convention only. The entry's signature proves only that a value has not
// changed since the entry was signed, not that it was true when written. Make
// no assumptions about a value's contents unless your application generated
// it, for example a forge reading back fields it stamped under its own
// namespace, and never use custom fields to make security decisions.
type CustomFields map[string]string

// CustomFieldEntry represents RSL entry types that carry application-defined
// custom fields. It is a separate interface rather than an addition to Entry so
// that adding GetCustomField does not break existing Entry implementations:
// they continue to satisfy Entry, and simply do not satisfy CustomFieldEntry.
// Callers holding an Entry reach the fields with a type assertion.
type CustomFieldEntry interface {
	Entry

	// GetCustomField returns the value of the named custom field and whether
	// it is set. Fields are advisory only: gittuf never consults them during
	// verification. Treat values as untrusted input unless your application
	// generated them. See CustomFields.
	GetCustomField(key string) (string, bool)
}

// appendCustomFieldLines drops fields with empty values, validates the rest,
// then appends them to lines as "<key>: <value>" sorted by key so the encoding
// is deterministic and the resulting commit is stable across implementations.
func appendCustomFieldLines(lines []string, fields CustomFields) ([]string, error) {
	effective := make(CustomFields, len(fields))
	for key, value := range fields {
		if value == "" {
			continue // an empty value means the field is absent
		}
		effective[key] = value
	}
	if len(effective) == 0 {
		return lines, nil
	}
	if err := customfields.Validate(effective); err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(effective))
	for key := range effective {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	for _, key := range keys {
		lines = append(lines, fmt.Sprintf("%s: %s", key, effective[key]))
	}
	return lines, nil
}

func setCustomField(fields *CustomFields, key, value string) {
	if err := customfields.ValidateField(key, value); err != nil {
		return
	}
	if value == "" {
		// An empty value means the field is absent. A canonical writer never
		// emits one, and recording it would report a field as set whose value
		// cannot round-trip.
		return
	}
	if *fields == nil {
		*fields = CustomFields{}
	}
	if _, exists := (*fields)[key]; exists {
		return
	}
	if len(*fields) >= customfields.MaxCount {
		return
	}
	(*fields)[key] = value
}
