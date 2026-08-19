// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Package customfields defines the format for signed, application-defined
// custom fields carried by RSL entries.
//
// Custom fields are advisory only and must be treated as untrusted input.
// gittuf treats them as opaque metadata and never consults them during
// verification, so their presence, absence, or contents cannot change a
// verification outcome.
//
// Consumers should make no assumptions about a value's
// contents unless they generated the data themselves, for example a forge
// reading back fields it stamped under its own namespace, and must not use
// custom fields to make security decisions.
package customfields

import (
	"errors"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

const (
	Prefix = "custom."

	// MaxKeyLength bounds a key, Prefix included. A key must be fewer than
	// MaxKeyLength characters, so a key of exactly this length is rejected.
	MaxKeyLength = 250

	// MaxValueLength bounds a value. A value must be fewer than
	// MaxValueLength characters, so a value of exactly this length is
	// rejected.
	MaxValueLength = 500

	// MaxCount is the largest number of fields a field set may carry. Unlike
	// the length limits, it is inclusive: exactly MaxCount fields are valid.
	MaxCount = 20

	// ValuePunctuation is the punctuation a value may carry alongside ASCII
	// letters and digits. It covers common value shapes: emails and handles
	// (@), timestamps and URLs (:), base64 (=), and query strings (?, &, %, #,
	// ~), as well as internal spaces. It is exported so an application that
	// maps its own input into a value, such as a forge sanitizing a user
	// handle, can do so against this alphabet instead of restating it.
	ValuePunctuation = "-+./,()_ @:=%#?&~"
)

var (
	ErrInvalid = errors.New("invalid custom fields")

	// domainLabel matches a single lowercase DNS label.
	domainLabel = regexp.MustCompile(`^[a-z0-9]([a-z0-9-]*[a-z0-9])?$`)

	// nameSegment matches a Kubernetes-style name segment: lowercase, starting
	// and ending alphanumeric, with '.', '_', and '-' allowed in between.
	nameSegment = regexp.MustCompile(`^[a-z0-9]([a-z0-9._-]*[a-z0-9])?$`)
)

func validValueRune(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
		r >= '0' && r <= '9' || strings.ContainsRune(ValuePunctuation, r)
}

// Validate checks that the fields are well formed. A field set
// carries at most MaxCount fields.
func Validate(fields map[string]string) error {
	if len(fields) > MaxCount {
		return fmt.Errorf("%w: %d fields exceed the maximum of %d", ErrInvalid, len(fields), MaxCount)
	}
	for key, value := range fields {
		if !ValidKey(key) {
			return fmt.Errorf("%w: invalid key %q", ErrInvalid, key)
		}
		if !ValidValue(value) {
			return fmt.Errorf("%w: invalid value for %q", ErrInvalid, key)
		}
	}
	return nil
}

// ValidKey reports whether key is a well-formed field key: the Prefix, a
// lowercase DNS subdomain, a '/', and a name of at most 63 characters that
// starts and ends with a lowercase letter or digit and may carry '.', '_', and
// '-' in between. The whole key, Prefix included, must be fewer than
// MaxKeyLength characters.
func ValidKey(key string) bool {
	if !strings.HasPrefix(key, Prefix) || len(key) >= MaxKeyLength {
		return false
	}
	domain, name, ok := strings.Cut(key[len(Prefix):], "/")
	if !ok {
		return false
	}
	return validDomain(domain) && validNameSegment(name)
}

func validDomain(domain string) bool {
	if len(domain) == 0 || len(domain) > 253 {
		return false
	}
	for label := range strings.SplitSeq(domain, ".") {
		if len(label) > 63 || !domainLabel.MatchString(label) {
			return false
		}
	}
	return true
}

func validNameSegment(name string) bool {
	return len(name) > 0 && len(name) <= 63 && nameSegment.MatchString(name)
}

// ValidValue reports whether value is a well-formed field value: fewer than
// MaxValueLength characters, no leading or trailing spaces, and every rune an
// ASCII letter, digit, or one of the characters in ValuePunctuation. This is a
// check of a value's shape, so the empty string passes. Emptiness means the
// field is absent, which writers handle by dropping the field.
func ValidValue(value string) bool {
	if len(value) >= MaxValueLength {
		return false
	}
	// The RSL entry parser trims each value, so a leading or trailing space
	// would not round-trip and would desync from the signed bytes. Reject it
	// on write.
	if value != strings.TrimSpace(value) {
		return false
	}
	for _, character := range value {
		if !validValueRune(character) {
			return false
		}
	}
	return true
}

// AppendLines drops fields with empty values, validates the rest, then appends
// them to lines as "<key>: <value>" sorted by key so the encoding is
// deterministic and the resulting commit is stable across implementations.
func AppendLines(lines []string, fields map[string]string) ([]string, error) {
	effective := make(map[string]string, len(fields))
	for key, value := range fields {
		if value == "" {
			continue // an empty value means the field is absent
		}
		effective[key] = value
	}
	if len(effective) == 0 {
		return lines, nil
	}
	if err := Validate(effective); err != nil {
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
