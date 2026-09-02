// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package customfields

import (
	"fmt"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidate(t *testing.T) {
	t.Parallel()

	invalid := map[string]map[string]string{
		"missing namespace":    {"field": "value"},
		"no custom prefix":     {"example.com/field": "value"},
		"no name separator":    {"custom.example.com": "value"},
		"empty name":           {"custom.example.com/": "value"},
		"empty domain":         {"custom./field": "value"},
		"uppercase key":        {"custom.example.com/Field": "value"},
		"key with space":       {"custom.example.com/field name": "value"},
		"bad domain label":     {"custom.-bad.com/field": "value"},
		"name too long":        {"custom.example.com/" + strings.Repeat("a", 64): "value"},
		"key too long":         {"custom." + strings.Repeat("a.", 125) + "com/field": "value"},
		"multiline value":      {"custom.example.com/field": "first\nsecond"},
		"value leading space":  {"custom.example.com/field": " value"},
		"value trailing space": {"custom.example.com/field": "value "},
		"value too long":       {"custom.example.com/field": strings.Repeat("a", MaxValueLength)},
		"value with asterisk":  {"custom.example.com/field": "a*b"},
		"value with quote":     {"custom.example.com/field": `a"b`},
		"value with backslash": {"custom.example.com/field": `a\b`},
		"value with semicolon": {"custom.example.com/field": "a;b"},
		"value with angle":     {"custom.example.com/field": "<tag>"},
		"non-ascii value":      {"custom.example.com/field": "café"},
	}

	for name, fields := range invalid {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.ErrorIs(t, Validate(fields), ErrInvalid)
		})
	}

	valid := map[string]map[string]string{
		"ulid":                {"custom.example.com/repository": "01ARZ3NDEKTSV4RRFFQ69G5FAV"},
		"symbols":             {"custom.example.com/field": "v1.2.3-rc.1/build(42),ok"},
		"internal space":      {"custom.example.com/field": "hello world"},
		"underscore":          {"custom.example.com/field": "hello_world"},
		"single label domain": {"custom.example/field": "value"},
		"subdomains":          {"custom.a.b.example.com/x-y_z.1": "value"},
		"max name length":     {"custom.example.com/" + strings.Repeat("a", 63): "value"},
		"max value length":    {"custom.example.com/field": strings.Repeat("a", MaxValueLength-1)},
		"email":               {"custom.example.com/pusher": "paulo@example.com"},
		"url":                 {"custom.example.com/source": "https://github.com/gittuf/gittuf?tab=readme#usage"},
		"rfc3339 timestamp":   {"custom.example.com/pushed-at": "2026-08-18T10:04:05Z"},
		"base64":              {"custom.example.com/digest": "q1w2e3r4=="},
		"percent encoding":    {"custom.example.com/path": "a%20b"},
		"tilde":               {"custom.example.com/ref": "HEAD~1"},
		"handle with id":      {"custom.gitforge.com/pusher": "jane (01ARZ3NDEKTSV4RRFFQ69G5FAV)"},
		"template version":    {"custom.gitforge.com/policy-template-version": "2026-03-11.4"},
	}

	for name, fields := range valid {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			assert.NoError(t, Validate(fields))
		})
	}
}

func TestValidKey(t *testing.T) {
	t.Parallel()

	assert.True(t, ValidKey("custom.example.com/field"))
	assert.False(t, ValidKey("example.com/field"))
	assert.False(t, ValidKey("custom.example.com"))
	assert.False(t, ValidKey("custom."+strings.Repeat("a.", 125)+"com/field"))
}

func TestValidKeyLengthBoundary(t *testing.T) {
	t.Parallel()

	domain := strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 51)
	key := Prefix + domain + "/" + strings.Repeat("a", 63)
	require.Len(t, key, MaxKeyLength)

	assert.False(t, ValidKey(key))
	assert.True(t, ValidKey(key[:len(key)-1]))
}

func TestValidValue(t *testing.T) {
	t.Parallel()

	assert.True(t, ValidValue("paulo@example.com"))
	assert.True(t, ValidValue(""))
	assert.False(t, ValidValue(" padded "))
	assert.False(t, ValidValue("first\nsecond"))
	assert.False(t, ValidValue(strings.Repeat("a", MaxValueLength)))
}

func TestValidValueRune(t *testing.T) {
	t.Parallel()

	for _, r := range "azAZ09" + ValuePunctuation {
		assert.True(t, validValueRune(r), "expected %q to be valid", r)
	}
	for _, r := range "*\"\\;<>\n\t|$`é" {
		assert.False(t, validValueRune(r), "expected %q to be invalid", r)
	}
}

func TestValuePunctuationIsItselfAValidValue(t *testing.T) {
	t.Parallel()

	assert.True(t, ValidValue(ValuePunctuation))
}

func TestValidateRejectsTooManyFields(t *testing.T) {
	t.Parallel()

	fields := map[string]string{}
	for i := 0; i <= MaxCount; i++ {
		fields[fmt.Sprintf("custom.example.com/field-%d", i)] = "v"
	}
	require.Len(t, fields, MaxCount+1)
	assert.ErrorIs(t, Validate(fields), ErrInvalid)

	delete(fields, fmt.Sprintf("custom.example.com/field-%d", MaxCount))
	require.Len(t, fields, MaxCount)
	assert.NoError(t, Validate(fields))
}

func TestAppendLines(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		lines    []string
		fields   map[string]string
		expected []string
	}{
		"no fields returns lines unchanged": {
			lines:    []string{"ref: refs/heads/main"},
			expected: []string{"ref: refs/heads/main"},
		},
		"fields sorted by key": {
			lines: []string{"ref: refs/heads/main"},
			fields: map[string]string{
				"custom.example.com/zebra": "last",
				"custom.example.com/alpha": "first",
			},
			expected: []string{"ref: refs/heads/main", "custom.example.com/alpha: first", "custom.example.com/zebra: last"},
		},
		"empty value dropped": {
			fields: map[string]string{
				"custom.example.com/set":   "value",
				"custom.example.com/unset": "",
			},
			expected: []string{"custom.example.com/set: value"},
		},
		"only empty values leaves lines unchanged": {
			lines:    []string{"ref: refs/heads/main"},
			fields:   map[string]string{"custom.example.com/unset": ""},
			expected: []string{"ref: refs/heads/main"},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			lines, err := AppendLines(test.lines, test.fields)
			require.NoError(t, err)
			assert.Equal(t, test.expected, lines)
		})
	}
}

func TestAppendLinesRejectsInvalidFields(t *testing.T) {
	t.Parallel()

	tests := map[string]map[string]string{
		"missing prefix":  {"example.com/field": "value"},
		"missing name":    {"custom.example.com": "value"},
		"uppercase key":   {"custom.example.com/Field": "value"},
		"newline value":   {"custom.example.com/field": "first\nsecond"},
		"padded value":    {"custom.example.com/field": " value "},
		"oversized value": {"custom.example.com/field": strings.Repeat("a", MaxValueLength)},
	}

	for name, fields := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			lines, err := AppendLines([]string{"ref: refs/heads/main"}, fields)
			assert.ErrorIs(t, err, ErrInvalid)
			assert.Nil(t, lines)
		})
	}
}
