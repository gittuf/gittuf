// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"encoding/base64"
	"fmt"
	"math"
	"testing"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/stretchr/testify/assert"
)

func TestIsRelevantGittufRef(t *testing.T) {
	tests := map[string]struct {
		refName  string
		expected bool
	}{
		"non gittuf ref": {
			refName:  "refs/heads/main",
			expected: false,
		},
		"policy staging ref": {
			refName:  gittufPolicyStagingRef,
			expected: false,
		},
		"policy ref": {
			refName:  "refs/gittuf/policy",
			expected: true,
		},
		"rsl ref": {
			refName:  Ref,
			expected: true,
		},
	}

	for name, test := range tests {
		assert.Equal(t, test.expected, isRelevantGittufRef(test.refName), fmt.Sprintf("unexpected result in test '%s'", name))
	}
}

func TestReferenceEntryCreateCommitMessage(t *testing.T) {
	nonZeroHash, err := NewHash("abcdef12345678900987654321fedcbaabcdef12")
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		entry           *ReferenceEntry
		expectedMessage string
	}{
		"entry, fully resolved ref": {
			entry: &ReferenceEntry{
				RefName:  "refs/heads/main",
				TargetID: githash.ZeroHash,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, githash.ZeroHash.String()),
		},
		"entry, non-zero commit": {
			entry: &ReferenceEntry{
				RefName:  "refs/heads/main",
				TargetID: nonZeroHash,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12"),
		},
		"entry, fully resolved ref, small number": {
			entry: &ReferenceEntry{
				RefName:  "refs/heads/main",
				TargetID: githash.ZeroHash,
				Number:   1,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, githash.ZeroHash.String(), NumberKey, 1),
		},
		"entry, fully resolved ref, large number": {
			entry: &ReferenceEntry{
				RefName:  "refs/heads/main",
				TargetID: githash.ZeroHash,
				Number:   math.MaxUint64,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, githash.ZeroHash.String(), NumberKey, uint64(math.MaxUint64)),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, _ := test.entry.createCommitMessage(true)
			if !assert.Equal(t, test.expectedMessage, message) {
				t.Errorf("expected\n%s\n\ngot\n%s", test.expectedMessage, message)
			}
		})
	}
}

func TestAnnotationEntryCreateCommitMessage(t *testing.T) {
	tests := map[string]struct {
		entry           *AnnotationEntry
		expectedMessage string
	}{
		"annotation, no message": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        true,
				Message:     "",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "true"),
		},
		"annotation, with message": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        true,
				Message:     "message",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message")), EndMessage),
		},
		"annotation, with multi-line message": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        true,
				Message:     "message1\nmessage2",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message1\nmessage2")), EndMessage),
		},
		"annotation, no message, skip false": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, no message, skip false, multiple entry IDs": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []Hash{githash.ZeroHash, githash.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), EntryIDKey, githash.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, no message, small number": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        true,
				Message:     "",
				Number:      1,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "true", NumberKey, 1),
		},
		"annotation, no message, large number": {
			entry: &AnnotationEntry{
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        true,
				Message:     "",
				Number:      math.MaxUint64,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "true", NumberKey, uint64(math.MaxUint64)),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, err := test.entry.createCommitMessage(true)
			if err != nil {
				t.Fatal(err)
			}
			if !assert.Equal(t, test.expectedMessage, message) {
				t.Errorf("expected\n%s\n\ngot\n%s", test.expectedMessage, message)
			}
		})
	}
}

func TestPropagationEntryCreateCommitMessage(t *testing.T) {
	nonZeroHash, err := NewHash("abcdef12345678900987654321fedcbaabcdef12")
	if err != nil {
		t.Fatal(err)
	}

	upstreamRepository := "https://git.example.com/example/repository"

	tests := map[string]struct {
		entry           *PropagationEntry
		expectedMessage string
	}{
		"entry, fully resolved ref": {
			entry: &PropagationEntry{
				RefName:            "refs/heads/main",
				TargetID:           githash.ZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    githash.ZeroHash,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, githash.ZeroHash.String(), UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, githash.ZeroHash.String()),
		},
		"entry, non-zero commit": {
			entry: &PropagationEntry{
				RefName:            "refs/heads/main",
				TargetID:           nonZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    nonZeroHash,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12", UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, "abcdef12345678900987654321fedcbaabcdef12"),
		},
		"entry, fully resolved ref, small number": {
			entry: &PropagationEntry{
				RefName:            "refs/heads/main",
				TargetID:           githash.ZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    githash.ZeroHash,
				Number:             1,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %d", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, githash.ZeroHash.String(), UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, githash.ZeroHash.String(), NumberKey, 1),
		},
		"entry, fully resolved ref, large number": {
			entry: &PropagationEntry{
				RefName:            "refs/heads/main",
				TargetID:           githash.ZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    githash.ZeroHash,
				Number:             math.MaxUint64,
			},
			expectedMessage: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %d", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, githash.ZeroHash.String(), UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, githash.ZeroHash.String(), NumberKey, uint64(math.MaxUint64)),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			message, _ := test.entry.createCommitMessage(true)
			if !assert.Equal(t, test.expectedMessage, message) {
				t.Errorf("expected\n%s\n\ngot\n%s", test.expectedMessage, message)
			}
		})
	}
}

func TestParseRSLEntryText(t *testing.T) {
	nonZeroHash, err := NewHash("abcdef12345678900987654321fedcbaabcdef12")
	if err != nil {
		t.Fatal(err)
	}

	upstreamRepository := "https://git.example.com/example/repository"

	tests := map[string]struct {
		expectedEntry Entry
		expectedError error
		message       string
	}{
		"entry, fully resolved ref": {
			expectedEntry: &ReferenceEntry{
				ID:       githash.ZeroHash,
				RefName:  "refs/heads/main",
				TargetID: githash.ZeroHash,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, githash.ZeroHash.String()),
		},
		"entry, non-zero commit": {
			expectedEntry: &ReferenceEntry{
				ID:       githash.ZeroHash,
				RefName:  "refs/heads/main",
				TargetID: nonZeroHash,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12"),
		},
		"entry, missing header": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s: %s\n%s: %s", RefKey, "refs/heads/main", TargetIDKey, githash.ZeroHash.String()),
		},
		"entry, missing information": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s\n\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main"),
		},
		"annotation, no message": {
			expectedEntry: &AnnotationEntry{
				ID:          githash.ZeroHash,
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        true,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "true"),
		},
		"annotation, with message": {
			expectedEntry: &AnnotationEntry{
				ID:          githash.ZeroHash,
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        true,
				Message:     "message",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message")), EndMessage),
		},
		"annotation, with multi-line message": {
			expectedEntry: &AnnotationEntry{
				ID:          githash.ZeroHash,
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        true,
				Message:     "message1\nmessage2",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message1\nmessage2")), EndMessage),
		},
		"annotation, no message, skip false": {
			expectedEntry: &AnnotationEntry{
				ID:          githash.ZeroHash,
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, no message, skip false, multiple entry IDs": {
			expectedEntry: &AnnotationEntry{
				ID:          githash.ZeroHash,
				RSLEntryIDs: []Hash{githash.ZeroHash, githash.ZeroHash},
				Skip:        false,
				Message:     "",
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), EntryIDKey, githash.ZeroHash.String(), SkipKey, "false"),
		},
		"annotation, missing header": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s: %s\n%s: %s\n%s\n%s\n%s", EntryIDKey, githash.ZeroHash.String(), SkipKey, "true", BeginMessage, base64.StdEncoding.EncodeToString([]byte("message")), EndMessage),
		},
		"annotation, missing information": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s\n\n%s: %s", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String()),
		},
		"propagation entry, fully resolved ref": {
			expectedEntry: &PropagationEntry{
				ID:                 githash.ZeroHash,
				RefName:            "refs/heads/main",
				TargetID:           githash.ZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    githash.ZeroHash,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, githash.ZeroHash.String(), UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, githash.ZeroHash.String()),
		},
		"propagation entry, non-zero commit": {
			expectedEntry: &PropagationEntry{
				ID:                 githash.ZeroHash,
				RefName:            "refs/heads/main",
				TargetID:           nonZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    nonZeroHash,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12", UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, "abcdef12345678900987654321fedcbaabcdef12"),
		},
		"propagation entry, missing information": {
			expectedError: ErrInvalidRSLEntry,
			message:       fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12", UpstreamRepositoryKey, upstreamRepository),
		},
		"entry, with number": {
			expectedEntry: &ReferenceEntry{
				ID:       githash.ZeroHash,
				RefName:  "refs/heads/main",
				TargetID: nonZeroHash,
				Number:   42,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, "abcdef12345678900987654321fedcbaabcdef12", NumberKey, 42),
		},
		"annotation, with number": {
			expectedEntry: &AnnotationEntry{
				ID:          githash.ZeroHash,
				RSLEntryIDs: []Hash{githash.ZeroHash},
				Skip:        true,
				Number:      7,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", AnnotationEntryHeader, EntryIDKey, githash.ZeroHash.String(), SkipKey, "true", NumberKey, 7),
		},
		"propagation entry, with number": {
			expectedEntry: &PropagationEntry{
				ID:                 githash.ZeroHash,
				RefName:            "refs/heads/main",
				TargetID:           githash.ZeroHash,
				UpstreamRepository: upstreamRepository,
				UpstreamEntryID:    githash.ZeroHash,
				Number:             3,
			},
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %d", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, githash.ZeroHash.String(), UpstreamRepositoryKey, upstreamRepository, UpstreamEntryIDKey, githash.ZeroHash.String(), NumberKey, 3),
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			entry, err := parseRSLEntryText(githash.ZeroHash, test.message)
			if err != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else if !assert.Equal(t, test.expectedEntry, entry) {
				t.Errorf("expected\n%+v\n\ngot\n%+v", test.expectedEntry, entry)
			}
		})
	}
}

func TestParseRSLEntryTextRejectsMalformed(t *testing.T) {
	t.Parallel()

	zero := githash.ZeroHash.String()
	upstream := "https://git.example.com/example/repository"

	// Each message below is malformed in exactly one way and must be rejected
	// with ErrInvalidRSLEntry. The previous loose parser accepted most of these
	// (last-write-wins, any order, missing fields), so these are the adversarial
	// cases that motivate the state machine.
	tests := map[string]string{
		"reference, duplicate ref": fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s",
			ReferenceEntryHeader, RefKey, "refs/heads/main", RefKey, "refs/heads/other", TargetIDKey, zero),
		"reference, duplicate targetID": fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s",
			ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero, TargetIDKey, zero),
		"reference, duplicate number": fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d\n%s: %d",
			ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero, NumberKey, 1, NumberKey, 2),
		"reference, targetID before ref": fmt.Sprintf("%s\n\n%s: %s\n%s: %s",
			ReferenceEntryHeader, TargetIDKey, zero, RefKey, "refs/heads/main"),
		"reference, number before targetID": fmt.Sprintf("%s\n\n%s: %s\n%s: %d\n%s: %s",
			ReferenceEntryHeader, RefKey, "refs/heads/main", NumberKey, 1, TargetIDKey, zero),
		"reference, missing ref": fmt.Sprintf("%s\n\n%s: %s",
			ReferenceEntryHeader, TargetIDKey, zero),
		"reference, missing targetID": fmt.Sprintf("%s\n\n%s: %s",
			ReferenceEntryHeader, RefKey, "refs/heads/main"),
		"reference, line without colon": fmt.Sprintf("%s\n\n%s: %s\n%s: %s\ngarbage",
			ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero),
		"reference, header with trailing text": fmt.Sprintf("%s extra\n\n%s: %s\n%s: %s",
			ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero),
		"reference, non-blank second line": fmt.Sprintf("%s\nnot blank\n%s: %s\n%s: %s",
			ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero),
		"annotation, entryID after skip": fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s",
			AnnotationEntryHeader, EntryIDKey, zero, SkipKey, "true", EntryIDKey, zero),
		"annotation, duplicate skip": fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s",
			AnnotationEntryHeader, EntryIDKey, zero, SkipKey, "true", SkipKey, "false"),
		"annotation, skip before entryID": fmt.Sprintf("%s\n\n%s: %s\n%s: %s",
			AnnotationEntryHeader, SkipKey, "true", EntryIDKey, zero),
		"annotation, missing skip": fmt.Sprintf("%s\n\n%s: %s",
			AnnotationEntryHeader, EntryIDKey, zero),
		"annotation, missing entryID": fmt.Sprintf("%s\n\n%s: %s",
			AnnotationEntryHeader, SkipKey, "true"),
		"annotation, invalid skip value": fmt.Sprintf("%s\n\n%s: %s\n%s: %s",
			AnnotationEntryHeader, EntryIDKey, zero, SkipKey, "maybe"),
		"propagation, duplicate upstreamRepository": fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %s",
			PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero, UpstreamRepositoryKey, upstream, UpstreamRepositoryKey, upstream, UpstreamEntryIDKey, zero),
		"propagation, upstreamEntryID before upstreamRepository": fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s",
			PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero, UpstreamEntryIDKey, zero, UpstreamRepositoryKey, upstream),
		"propagation, missing upstreamEntryID": fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s",
			PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero, UpstreamRepositoryKey, upstream),
	}

	for name, message := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry, err := parseRSLEntryText(githash.ZeroHash, message)
			assert.True(t, entry == nil, "expected a true nil Entry, not a typed nil pointer")
			assert.ErrorIs(t, err, ErrInvalidRSLEntry)
		})
	}
}

func TestParseRSLEntryTextForwardCompatibility(t *testing.T) {
	t.Parallel()

	zero := githash.ZeroHash.String()
	upstream := "https://git.example.com:8443/example/repository"

	tests := map[string]struct {
		message       string
		expectedEntry Entry
	}{
		"reference, unknown trailing key ignored": {
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\nfutureField: someValue",
				ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero),
			expectedEntry: &ReferenceEntry{ID: githash.ZeroHash, RefName: "refs/heads/main", TargetID: githash.ZeroHash},
		},
		"reference, unknown leading key ignored": {
			message: fmt.Sprintf("%s\n\nfutureField: someValue\n%s: %s\n%s: %s",
				ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero),
			expectedEntry: &ReferenceEntry{ID: githash.ZeroHash, RefName: "refs/heads/main", TargetID: githash.ZeroHash},
		},
		"annotation, unknown key ignored between fields": {
			message: fmt.Sprintf("%s\n\n%s: %s\nfutureField: someValue\n%s: %s",
				AnnotationEntryHeader, EntryIDKey, zero, SkipKey, "false"),
			expectedEntry: &AnnotationEntry{ID: githash.ZeroHash, RSLEntryIDs: []Hash{githash.ZeroHash}, Skip: false},
		},
		"propagation, upstreamRepository with colons preserved": {
			message: fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s",
				PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, zero, UpstreamRepositoryKey, upstream, UpstreamEntryIDKey, zero),
			expectedEntry: &PropagationEntry{ID: githash.ZeroHash, RefName: "refs/heads/main", TargetID: githash.ZeroHash, UpstreamRepository: upstream, UpstreamEntryID: githash.ZeroHash},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry, err := parseRSLEntryText(githash.ZeroHash, test.message)
			assert.NoError(t, err)
			assert.Equal(t, test.expectedEntry, entry)
		})
	}
}

const (
	fuzzZeroHash    = "0000000000000000000000000000000000000000"
	fuzzNonZeroHash = "abcdef12345678900987654321fedcbaabcdef12"
)

func FuzzParseRSLEntryText(f *testing.F) {
	f.Add("")
	f.Add("not an entry")
	f.Add(fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, fuzzZeroHash))
	f.Add(fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, fuzzZeroHash, SkipKey, "true"))
	f.Add(fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, fuzzZeroHash, UpstreamRepositoryKey, "https://git.example.com:8443/repo", UpstreamEntryIDKey, fuzzNonZeroHash))

	f.Fuzz(func(_ *testing.T, text string) {
		_, _ = parseRSLEntryText(githash.ZeroHash, text)
	})
}

func FuzzParseReferenceEntryText(f *testing.F) {
	f.Add("")
	f.Add(fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, fuzzZeroHash))
	f.Add(fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, fuzzNonZeroHash, NumberKey, 5))
	f.Add(fmt.Sprintf("%s\n\n%s: %s\n%s: %s", ReferenceEntryHeader, TargetIDKey, fuzzZeroHash, RefKey, "refs/heads/main"))

	f.Fuzz(func(_ *testing.T, text string) {
		_, _ = parseReferenceEntryText(githash.ZeroHash, text)
	})
}

func FuzzParseAnnotationEntryText(f *testing.F) {
	f.Add("")
	f.Add(fmt.Sprintf("%s\n\n%s: %s\n%s: %s", AnnotationEntryHeader, EntryIDKey, fuzzZeroHash, SkipKey, "true"))
	f.Add(fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s\n%s\n%s", AnnotationEntryHeader, EntryIDKey, fuzzZeroHash, EntryIDKey, fuzzNonZeroHash, SkipKey, "false", BeginMessage, base64.StdEncoding.EncodeToString([]byte("note")), EndMessage))

	f.Fuzz(func(_ *testing.T, text string) {
		_, _ = parseAnnotationEntryText(githash.ZeroHash, text)
	})
}

func FuzzParsePropagationEntryText(f *testing.F) {
	f.Add("")
	f.Add(fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, fuzzZeroHash, UpstreamRepositoryKey, "https://git.example.com:8443/repo", UpstreamEntryIDKey, fuzzNonZeroHash))
	f.Add(fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %d", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, fuzzZeroHash, UpstreamRepositoryKey, "https://git.example.com/repo", UpstreamEntryIDKey, fuzzNonZeroHash, NumberKey, 9))

	f.Fuzz(func(_ *testing.T, text string) {
		_, _ = parsePropagationEntryText(githash.ZeroHash, text)
	})
}

func BenchmarkParseReferenceEntryText(b *testing.B) {
	text := fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, fuzzNonZeroHash, NumberKey, 42)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := parseReferenceEntryText(githash.ZeroHash, text); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseAnnotationEntryText(b *testing.B) {
	text := fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %d\n%s\n%s\n%s", AnnotationEntryHeader, EntryIDKey, fuzzZeroHash, EntryIDKey, fuzzNonZeroHash, SkipKey, "true", NumberKey, 42, BeginMessage, base64.StdEncoding.EncodeToString([]byte("annotation message")), EndMessage)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := parseAnnotationEntryText(githash.ZeroHash, text); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseAnnotationEntryTextNoMessage(b *testing.B) {
	text := fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", AnnotationEntryHeader, EntryIDKey, fuzzZeroHash, SkipKey, "true", NumberKey, 42)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := parseAnnotationEntryText(githash.ZeroHash, text); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParsePropagationEntryText(b *testing.B) {
	text := fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %s\n%s: %s\n%s: %d", PropagationEntryHeader, RefKey, "refs/heads/main", TargetIDKey, fuzzNonZeroHash, UpstreamRepositoryKey, "https://git.example.com:8443/example/repository", UpstreamEntryIDKey, fuzzZeroHash, NumberKey, 42)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := parsePropagationEntryText(githash.ZeroHash, text); err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkParseRSLEntryText(b *testing.B) {
	text := fmt.Sprintf("%s\n\n%s: %s\n%s: %s\n%s: %d", ReferenceEntryHeader, RefKey, "refs/heads/main", TargetIDKey, fuzzNonZeroHash, NumberKey, 42)

	b.ReportAllocs()
	for b.Loop() {
		if _, err := parseRSLEntryText(githash.ZeroHash, text); err != nil {
			b.Fatal(err)
		}
	}
}
