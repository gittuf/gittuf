// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"strings"
	"testing"

	"github.com/gittuf/gittuf/pkg/customfields"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAppendCustomFieldLines(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		lines    []string
		fields   CustomFields
		expected []string
	}{
		"no fields returns lines unchanged": {
			lines:    []string{"ref: refs/heads/main"},
			expected: []string{"ref: refs/heads/main"},
		},
		"fields sorted by key": {
			lines: []string{"ref: refs/heads/main"},
			fields: CustomFields{
				"custom.example.com/zebra": "last",
				"custom.example.com/alpha": "first",
			},
			expected: []string{"ref: refs/heads/main", "custom.example.com/alpha: first", "custom.example.com/zebra: last"},
		},
		"empty value dropped": {
			fields: CustomFields{
				"custom.example.com/set":   "value",
				"custom.example.com/unset": "",
			},
			expected: []string{"custom.example.com/set: value"},
		},
		"only empty values leaves lines unchanged": {
			lines:    []string{"ref: refs/heads/main"},
			fields:   CustomFields{"custom.example.com/unset": ""},
			expected: []string{"ref: refs/heads/main"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			lines, err := appendCustomFieldLines(test.lines, test.fields)
			require.NoError(t, err)
			assert.Equal(t, test.expected, lines)
		})
	}
}

func TestAppendCustomFieldLinesRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	tests := map[string]CustomFields{
		"missing prefix":  {"example.com/field": "value"},
		"missing name":    {"custom.example.com": "value"},
		"uppercase key":   {"custom.example.com/Field": "value"},
		"newline value":   {"custom.example.com/field": "first\nsecond"},
		"padded value":    {"custom.example.com/field": " value "},
		"oversized value": {"custom.example.com/field": strings.Repeat("a", customfields.MaxValueLength)},
	}

	for name, fields := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			lines, err := appendCustomFieldLines([]string{"ref: refs/heads/main"}, fields)
			assert.ErrorIs(t, err, customfields.ErrInvalid)
			assert.Nil(t, lines)
		})
	}
}
