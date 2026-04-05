// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestStatus(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd) //nolint:errcheck

	// NOTE: we don't use traditional methods like WriteBlob or TreeBuilder so
	// we can more closely simulate user actions, such as with updating index,
	// etc.

	filename := "foo"
	filename2 := "bar"

	if err := os.WriteFile(filename, []byte("foo"), 0o644); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	statuses, err := repo.Status()
	assert.Nil(t, err)
	assert.Equal(t, map[string]FileStatus{filename: {X: StatusCodeUntracked, Y: StatusCodeUntracked}}, statuses)

	// Add item to index
	if _, err := repo.executor("add", filename).executeString(); err != nil {
		t.Fatal(err)
	}

	statuses, err = repo.Status()
	assert.Nil(t, err)
	assert.Equal(t, map[string]FileStatus{filename: {X: StatusCodeAdded, Y: StatusCodeUnmodified}}, statuses)

	// Modify file that has been staged
	if err := os.WriteFile(filename, []byte("bar"), 0o644); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	statuses, err = repo.Status()
	assert.Nil(t, err)
	assert.Equal(t, map[string]FileStatus{filename: {X: StatusCodeAdded, Y: StatusCodeModified}}, statuses)

	// Add and commit
	if _, err := repo.executor("add", filename).executeString(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.executor("commit", "-m", "Commit\n").executeString(); err != nil {
		t.Fatal(err)
	}

	statuses, err = repo.Status()
	assert.Nil(t, err)
	assert.Empty(t, statuses)

	// Modify file again
	if err := os.WriteFile(filename, []byte("foo"), 0o644); err != nil { //nolint:gosec
		t.Fatal(err)
	}

	statuses, err = repo.Status()
	assert.Nil(t, err)
	assert.Equal(t, map[string]FileStatus{filename: {X: StatusCodeModified, Y: StatusCodeUnmodified}}, statuses)

	// Remove file
	if err := os.Remove(filename); err != nil {
		t.Fatal(err)
	}

	statuses, err = repo.Status()
	assert.Nil(t, err)
	assert.Equal(t, map[string]FileStatus{filename: {X: StatusCodeDeleted, Y: StatusCodeUnmodified}}, statuses)

	// Commit
	if _, err := repo.executor("add", filename).executeString(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.executor("commit", "-m", "Commit\n", "--allow-empty").executeString(); err != nil {
		t.Fatal(err)
	}

	statuses, err = repo.Status()
	assert.Nil(t, err)
	assert.Empty(t, statuses)

	// Add two files, commit, then make one a symlink to the other
	if err := os.WriteFile(filename, []byte("foo"), 0o644); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	if err := os.WriteFile(filename2, []byte("foo"), 0o644); err != nil { //nolint:gosec
		t.Fatal(err)
	}
	if _, err := repo.executor("add", filename, filename2).executeString(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.executor("commit", "-m", "Commit\n").executeString(); err != nil {
		t.Fatal(err)
	}

	statuses, err = repo.Status()
	assert.Nil(t, err)
	assert.Empty(t, statuses)

	if err := os.Remove(filename2); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(filename, filename2); err != nil {
		t.Fatal(err)
	}

	statuses, err = repo.Status()
	assert.Nil(t, err)
	assert.Equal(t, map[string]FileStatus{filename2: {X: StatusCodeTypeChanged, Y: StatusCodeUnmodified}}, statuses)

	// Add and commit
	if _, err := repo.executor("add", filename2).executeString(); err != nil {
		t.Fatal(err)
	}
	if _, err := repo.executor("commit", "-m", "Commit\n").executeString(); err != nil {
		t.Fatal(err)
	}

	statuses, err = repo.Status()
	assert.Nil(t, err)
	assert.Empty(t, statuses)
}

func TestStatusCodeString(t *testing.T) {
	tests := []struct {
		name     string
		code     StatusCode
		expected string
	}{
		{"Unmodified", StatusCodeUnmodified, " "},
		{"Modified", StatusCodeModified, "M"},
		{"TypeChanged", StatusCodeTypeChanged, "T"},
		{"Added", StatusCodeAdded, "A"},
		{"Deleted", StatusCodeDeleted, "D"},
		{"Renamed", StatusCodeRenamed, "R"},
		{"Copied", StatusCodeCopied, "C"},
		{"UpdatedUnmerged", StatusCodeUpdatedUnmerged, "U"},
		{"Untracked", StatusCodeUntracked, "?"},
		{"Ignored", StatusCodeIgnored, "!"},
		{"Invalid", StatusCode(999), "invalid-code"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.code.String()
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("all codes have string representation", func(t *testing.T) {
		allCodes := []StatusCode{
			StatusCodeUnmodified,
			StatusCodeModified,
			StatusCodeTypeChanged,
			StatusCodeAdded,
			StatusCodeDeleted,
			StatusCodeRenamed,
			StatusCodeCopied,
			StatusCodeUpdatedUnmerged,
			StatusCodeUntracked,
			StatusCodeIgnored,
		}

		for _, code := range allCodes {
			str := code.String()
			assert.NotEmpty(t, str)
		}
	})
}

func TestNewStatusCodeFromByte(t *testing.T) {
	tests := []struct {
		name        string
		input       byte
		expected    StatusCode
		expectError bool
	}{
		{"Unmodified", ' ', StatusCodeUnmodified, false},
		{"Modified", 'M', StatusCodeModified, false},
		{"TypeChanged", 'T', StatusCodeTypeChanged, false},
		{"Added", 'A', StatusCodeAdded, false},
		{"Deleted", 'D', StatusCodeDeleted, false},
		{"Copied", 'C', StatusCodeCopied, false},
		{"UpdatedUnmerged", 'U', StatusCodeUpdatedUnmerged, false},
		{"Untracked", '?', StatusCodeUntracked, false},
		{"Ignored", '!', StatusCodeIgnored, false},
		{"Invalid", 'X', 0, true},
		{"InvalidNumber", '1', 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result, err := NewStatusCodeFromByte(tt.input)
			if tt.expectError {
				assert.NotNil(t, err)
				assert.Equal(t, ErrInvalidStatusCode, err)
			} else {
				assert.Nil(t, err)
				assert.Equal(t, tt.expected, result)
			}
		})
	}

	t.Run("invalid bytes return error", func(t *testing.T) {
		invalidBytes := []byte{'R', 'Y', 'Z', '2', '@', '#'}
		for _, b := range invalidBytes {
			_, err := NewStatusCodeFromByte(b)
			assert.ErrorIs(t, err, ErrInvalidStatusCode)
		}
	})
}

func TestFileStatusUntracked(t *testing.T) {
	tests := []struct {
		name     string
		status   FileStatus
		expected bool
	}{
		{"X is untracked", FileStatus{X: StatusCodeUntracked, Y: StatusCodeUnmodified}, true},
		{"Y is untracked", FileStatus{X: StatusCodeModified, Y: StatusCodeUntracked}, true},
		{"Both untracked", FileStatus{X: StatusCodeUntracked, Y: StatusCodeUntracked}, true},
		{"Neither untracked", FileStatus{X: StatusCodeModified, Y: StatusCodeAdded}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := tt.status.Untracked()
			assert.Equal(t, tt.expected, result)
		})
	}

	t.Run("various status combinations", func(t *testing.T) {
		statuses := []FileStatus{
			{X: StatusCodeUntracked, Y: StatusCodeUntracked},
			{X: StatusCodeModified, Y: StatusCodeUntracked},
			{X: StatusCodeUntracked, Y: StatusCodeModified},
			{X: StatusCodeModified, Y: StatusCodeModified},
			{X: StatusCodeAdded, Y: StatusCodeUnmodified},
			{X: StatusCodeDeleted, Y: StatusCodeUnmodified},
		}

		for _, status := range statuses {
			isUntracked := status.Untracked()
			if status.X == StatusCodeUntracked || status.Y == StatusCodeUntracked {
				assert.True(t, isUntracked)
			} else {
				assert.False(t, isUntracked)
			}
		}
	})
}

func TestStatusWithEmptyRepository(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd) //nolint:errcheck

	// Check status on empty repository
	statuses, err := repo.Status()
	assert.Nil(t, err)
	assert.Empty(t, statuses)
}
