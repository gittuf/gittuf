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

func TestValidateFieldKey(t *testing.T) {
	t.Parallel()

	assert.NoError(t, ValidateField("custom.example.com/field", "value"))
	assert.ErrorIs(t, ValidateField("example.com/field", "value"), ErrInvalid)
	assert.ErrorIs(t, ValidateField("custom.example.com", "value"), ErrInvalid)
	assert.ErrorIs(t, ValidateField("custom."+strings.Repeat("a.", 125)+"com/field", "value"), ErrInvalid)
}

func TestValidateFieldKeyLengthBoundary(t *testing.T) {
	t.Parallel()

	domain := strings.Repeat("a", 63) + "." + strings.Repeat("a", 63) + "." + strings.Repeat("a", 51)
	key := Prefix + domain + "/" + strings.Repeat("a", 63)
	require.Len(t, key, MaxKeyLength)

	assert.ErrorIs(t, ValidateField(key, "value"), ErrInvalid)
	assert.NoError(t, ValidateField(key[:len(key)-1], "value"))
}

func TestValidateFieldValue(t *testing.T) {
	t.Parallel()

	const key = "custom.example.com/field"

	assert.NoError(t, ValidateField(key, "paulo@example.com"))
	assert.NoError(t, ValidateField(key, ""))
	assert.ErrorIs(t, ValidateField(key, " padded "), ErrInvalid)
	assert.ErrorIs(t, ValidateField(key, "first\nsecond"), ErrInvalid)
	assert.ErrorIs(t, ValidateField(key, strings.Repeat("a", MaxValueLength)), ErrInvalid)
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

	assert.NoError(t, ValidateField("custom.example.com/field", ValuePunctuation))
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
