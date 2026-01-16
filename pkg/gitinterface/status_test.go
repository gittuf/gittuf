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
