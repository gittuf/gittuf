// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"fmt"
	"slices"
	"strconv"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBulkReferenceEntryCreateCommitMessage(t *testing.T) {
	t.Parallel()

	nonZeroHash, err := NewHash("abcdef12345678900987654321fedcbaabcdef12")
	if err != nil {
		t.Fatal(err)
	}
	zero := githash.ZeroHash.String()

	tests := map[string]struct {
		entry           *BulkReferenceEntry
		expectedMessage string
		expectedErrors  []error
	}{
		"two updates": {
			entry: &BulkReferenceEntry{
				Updates: []ReferenceUpdate{
					{RefName: "refs/heads/main", TargetID: githash.ZeroHash},
					{RefName: "refs/heads/feature", TargetID: nonZeroHash},
				},
				Number: 5,
			},
			expectedMessage: BulkReferenceEntryHeader + "\n\nrefs/heads/main: " + zero + "\nrefs/heads/feature: " + nonZeroHash.String() + "\n\nnumber: 5",
		},
		"single update is allowed by the codec": {
			entry: &BulkReferenceEntry{
				Updates: []ReferenceUpdate{{RefName: "refs/heads/main", TargetID: githash.ZeroHash}},
				Number:  1,
			},
			expectedMessage: fmt.Sprintf("%s\n\nrefs/heads/main: %s\n\n%s: %d", BulkReferenceEntryHeader, zero, NumberKey, 1),
		},
		"no number": {
			entry:          NewBulkReferenceEntry([]ReferenceUpdate{{RefName: "refs/heads/main", TargetID: githash.ZeroHash}}),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"no updates": {
			entry:          NewBulkReferenceEntry(nil),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"duplicate ref": {
			entry: &BulkReferenceEntry{
				Updates: []ReferenceUpdate{
					{RefName: "refs/heads/main", TargetID: githash.ZeroHash},
					{RefName: "refs/heads/main", TargetID: nonZeroHash},
				},
				Number: 2,
			},
			expectedErrors: []error{ErrDuplicateReferenceInBulkEntry, ErrInvalidRSLEntry},
		},
		"gittuf ref": {
			entry: &BulkReferenceEntry{
				Updates: []ReferenceUpdate{
					{RefName: "refs/heads/main", TargetID: githash.ZeroHash},
					{RefName: "refs/gittuf/policy", TargetID: nonZeroHash},
				},
				Number: 2,
			},
			expectedErrors: []error{ErrGittufReferenceInBulkEntry, ErrInvalidRSLEntry},
		},
		"unqualified ref": {
			entry:          &BulkReferenceEntry{Updates: []ReferenceUpdate{{RefName: "main", TargetID: githash.ZeroHash}}, Number: 1},
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"ref with surrounding whitespace": {
			entry:          &BulkReferenceEntry{Updates: []ReferenceUpdate{{RefName: " refs/heads/main", TargetID: githash.ZeroHash}}, Number: 1},
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"ref with line break": {
			entry:          &BulkReferenceEntry{Updates: []ReferenceUpdate{{RefName: "refs/heads/main\nrefs/heads/other: " + zero, TargetID: githash.ZeroHash}}, Number: 1},
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			message, err := test.entry.createCommitMessage(true)
			if len(test.expectedErrors) != 0 {
				for _, expected := range test.expectedErrors {
					assert.ErrorIs(t, err, expected)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expectedMessage, message)
		})
	}
}

func TestBulkReferenceEntryCreateCommitMessageIgnoresIncludeNumber(t *testing.T) {
	t.Parallel()

	entry := &BulkReferenceEntry{
		Updates: []ReferenceUpdate{{RefName: "refs/heads/main", TargetID: githash.ZeroHash}},
		Number:  3,
	}

	withNumber, err := entry.createCommitMessage(true)
	require.NoError(t, err)
	withoutNumber, err := entry.createCommitMessage(false)
	require.NoError(t, err)
	assert.Equal(t, withNumber, withoutNumber)
	assert.Contains(t, withoutNumber, fmt.Sprintf("\n\n%s: 3", NumberKey))
}

func TestParseBulkReferenceEntryText(t *testing.T) {
	t.Parallel()

	nonZeroHash, err := NewHash("abcdef12345678900987654321fedcbaabcdef12")
	if err != nil {
		t.Fatal(err)
	}
	sha256Hash, err := NewHash("abcdef12345678900987654321fedcbaabcdef12345678900987654321fedcba")
	if err != nil {
		t.Fatal(err)
	}
	zero := githash.ZeroHash.String()
	body := func(lines ...string) string {
		return BulkReferenceEntryHeader + "\n\n" + strings.Join(lines, "\n")
	}

	tests := map[string]struct {
		message        string
		expectedEntry  *BulkReferenceEntry
		expectedErrors []error
	}{
		"two updates": {
			message: body("refs/heads/main: "+zero, "refs/heads/feature: "+nonZeroHash.String(), "", "number: 12"),
			expectedEntry: &BulkReferenceEntry{
				ID: githash.ZeroHash,
				Updates: []ReferenceUpdate{
					{RefName: "refs/heads/main", TargetID: githash.ZeroHash},
					{RefName: "refs/heads/feature", TargetID: nonZeroHash},
				},
				Number: 12,
			},
		},
		"single update": {
			message: body("refs/heads/main: "+zero, "", "number: 1"),
			expectedEntry: &BulkReferenceEntry{
				ID:      githash.ZeroHash,
				Updates: []ReferenceUpdate{{RefName: "refs/heads/main", TargetID: githash.ZeroHash}},
				Number:  1,
			},
		},
		"sha256 target ID": {
			message: body("refs/heads/main: "+sha256Hash.String(), "", "number: 1"),
			expectedEntry: &BulkReferenceEntry{
				ID:      githash.ZeroHash,
				Updates: []ReferenceUpdate{{RefName: "refs/heads/main", TargetID: sha256Hash}},
				Number:  1,
			},
		},
		"mixed whitespace around keys and values": {
			message: body("  refs/heads/main  :   "+zero+"  ", "\trefs/tags/v1:\t"+nonZeroHash.String(), "", "  number :  4 "),
			expectedEntry: &BulkReferenceEntry{
				ID: githash.ZeroHash,
				Updates: []ReferenceUpdate{
					{RefName: "refs/heads/main", TargetID: githash.ZeroHash},
					{RefName: "refs/tags/v1", TargetID: nonZeroHash},
				},
				Number: 4,
			},
		},
		"trailing blank lines after the number": {
			message: body("refs/heads/main: "+zero, "", "number: 1", "", ""),
			expectedEntry: &BulkReferenceEntry{
				ID:      githash.ZeroHash,
				Updates: []ReferenceUpdate{{RefName: "refs/heads/main", TargetID: githash.ZeroHash}},
				Number:  1,
			},
		},
		"missing number": {
			message:        body("refs/heads/main: " + zero),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"blank separator but no number": {
			message:        body("refs/heads/main: "+zero, ""),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"no blank line before the number": {
			message:        body("refs/heads/main: "+zero, "number: 1"),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"number before the separator": {
			message:        body("number: 1", "", "refs/heads/main: "+zero),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"two number lines": {
			message:        body("refs/heads/main: "+zero, "", "number: 1", "number: 2"),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"update line after the number": {
			message:        body("refs/heads/main: "+zero, "", "number: 1", "refs/heads/feature: "+zero),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"two blank lines before the number": {
			message:        body("refs/heads/main: "+zero, "", "", "number: 1"),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"blank line between update lines": {
			message:        body("refs/heads/main: "+zero, "", "refs/heads/feature: "+zero, "", "number: 1"),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"leading blank line in the body": {
			message:        body("", "refs/heads/main: "+zero, "", "number: 1"),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"no updates, only a number": {
			message:        body("number: 1"),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"line without a colon": {
			message:        body("refs/heads/main: "+zero, "not a key value line", "", "number: 1"),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"uppercase number key": {
			message:        body("refs/heads/main: "+zero, "", "Number: 1"),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"legacy ref key": {
			message:        body("ref: refs/heads/main", "targetID: "+zero, "", "number: 1"),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"duplicate ref": {
			message:        body("refs/heads/main: "+zero, "refs/heads/main: "+nonZeroHash.String(), "", "number: 1"),
			expectedErrors: []error{ErrDuplicateReferenceInBulkEntry, ErrInvalidRSLEntry},
		},
		"gittuf ref": {
			message:        body("refs/gittuf/policy: "+zero, "", "number: 1"),
			expectedErrors: []error{ErrGittufReferenceInBulkEntry, ErrInvalidRSLEntry},
		},
		"unqualified ref": {
			message:        body("main: "+zero, "", "number: 1"),
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"malformed hash": {
			message: body("refs/heads/main: zzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzzz", "", "number: 1"),
			// Hash errors do not wrap ErrInvalidRSLEntry anywhere in the
			// package, so only the hash sentinel is asserted here.
			expectedErrors: []error{ErrInvalidHashEncoding},
		},
		"non numeric number": {
			message:        body("refs/heads/main: "+zero, "", "number: five"),
			expectedErrors: []error{strconv.ErrSyntax},
		},
		"header with trailing text": {
			message:        BulkReferenceEntryHeader + " v2\n\nrefs/heads/main: " + zero + "\n\nnumber: 1",
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
		"missing blank line after the header": {
			message:        BulkReferenceEntryHeader + "\nrefs/heads/main: " + zero + "\n\nnumber: 1",
			expectedErrors: []error{ErrInvalidRSLEntry},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			entry, err := parseRSLEntryText(githash.ZeroHash, test.message)
			if len(test.expectedErrors) != 0 {
				for _, expected := range test.expectedErrors {
					assert.ErrorIs(t, err, expected)
				}
				return
			}
			assert.NoError(t, err)
			assert.Equal(t, test.expectedEntry, entry)
		})
	}
}

func TestBulkReferenceEntryRoundTrip(t *testing.T) {
	t.Parallel()

	nonZeroHash, err := NewHash("abcdef12345678900987654321fedcbaabcdef12")
	if err != nil {
		t.Fatal(err)
	}

	entry := &BulkReferenceEntry{
		ID: githash.ZeroHash,
		Updates: []ReferenceUpdate{
			{RefName: "refs/heads/main", TargetID: githash.ZeroHash},
			{RefName: "refs/heads/feature", TargetID: nonZeroHash},
		},
		Number: 7,
	}
	message, err := entry.createCommitMessage(true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, fmt.Sprintf("%s\n\nrefs/heads/main: %s\nrefs/heads/feature: %s\n\n%s: 7", BulkReferenceEntryHeader, githash.ZeroHash.String(), nonZeroHash.String(), NumberKey), message)

	parsed, err := parseRSLEntryText(githash.ZeroHash, message)
	assert.NoError(t, err)
	assert.Equal(t, entry, parsed)
}

func TestReferenceUpdaters(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		entry            Entry
		expectedRefNames []string
		expectedError    error
	}{
		"reference entry": {
			entry:            &ReferenceEntry{RefName: "refs/heads/main", TargetID: githash.ZeroHash},
			expectedRefNames: []string{"refs/heads/main"},
		},
		"propagation entry": {
			entry:            NewPropagationEntry("refs/heads/main", githash.ZeroHash, "https://git.example.com/repo", githash.ZeroHash),
			expectedRefNames: []string{"refs/heads/main"},
		},
		"annotation entry": {
			entry: &AnnotationEntry{},
		},
		"bulk reference entry": {
			entry: &BulkReferenceEntry{
				ID: githash.ZeroHash,
				Updates: []ReferenceUpdate{
					{RefName: "refs/heads/main", TargetID: githash.ZeroHash},
					{RefName: "refs/heads/feature", TargetID: githash.ZeroHash},
					{RefName: "refs/tags/v1", TargetID: githash.ZeroHash},
				},
			},
			expectedRefNames: []string{"refs/heads/main", "refs/heads/feature", "refs/tags/v1"},
		},
		"unknown entry type": {
			entry:         &fakeEntry{},
			expectedError: ErrUnknownRSLEntryType,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			updaters, err := referenceUpdaters(test.entry)
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
			} else {
				assert.NoError(t, err)
				assert.Equal(t, test.expectedRefNames, refNames(updaters))
			}

			newestFirst, err := referenceUpdatersNewestFirst(test.entry)
			if test.expectedError != nil {
				assert.ErrorIs(t, err, test.expectedError)
				return
			}
			assert.NoError(t, err)
			reversed := slices.Clone(test.expectedRefNames)
			slices.Reverse(reversed)
			assert.Equal(t, reversed, refNames(newestFirst))
		})
	}
}

func TestReferenceUpdatersViewsAreIndependent(t *testing.T) {
	t.Parallel()

	entry := &BulkReferenceEntry{
		ID: githash.ZeroHash,
		Updates: []ReferenceUpdate{
			{RefName: "refs/heads/main", TargetID: githash.ZeroHash},
			{RefName: "refs/heads/feature", TargetID: githash.ZeroHash},
		},
	}

	updaters, err := referenceUpdaters(entry)
	assert.NoError(t, err)
	slices.Reverse(updaters)

	again, err := referenceUpdaters(entry)
	assert.NoError(t, err)
	assert.Equal(t, []string{"refs/heads/main", "refs/heads/feature"}, refNames(again))
}

func refNames(updaters []ReferenceUpdaterEntry) []string {
	if updaters == nil {
		return nil
	}

	names := make([]string, 0, len(updaters))
	for _, updater := range updaters {
		names = append(names, updater.GetRefName())
	}
	return names
}

type fakeEntry struct{}

func (f *fakeEntry) GetID() githash.Hash                      { return githash.ZeroHash }
func (f *fakeEntry) Commit(gitstore.Storer, bool) error       { return nil }
func (f *fakeEntry) GetNumber() uint64                        { return 0 }
func (f *fakeEntry) createCommitMessage(bool) (string, error) { return "", nil }

func FuzzParseBulkReferenceEntryText(f *testing.F) {
	f.Add("")
	f.Add(fmt.Sprintf("%s\n\nrefs/heads/main: %s\n\n%s: 1", BulkReferenceEntryHeader, fuzzZeroHash, NumberKey))
	f.Add(fmt.Sprintf("%s\n\nrefs/heads/main: %s\nrefs/heads/feature: %s\n\n%s: 3", BulkReferenceEntryHeader, fuzzZeroHash, fuzzNonZeroHash, NumberKey))
	f.Add(fmt.Sprintf("%s\n\nrefs/heads/main: %s\n%s: 3", BulkReferenceEntryHeader, fuzzZeroHash, NumberKey))
	f.Add(fmt.Sprintf("%s\n\n%s: refs/heads/main\n%s: %s", BulkReferenceEntryHeader, RefKey, TargetIDKey, fuzzZeroHash))

	f.Fuzz(func(_ *testing.T, text string) {
		_, _ = parseBulkReferenceEntryText(githash.ZeroHash, text)
	})
}
