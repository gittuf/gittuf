// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"os"
	"path"
	"strings"
	"testing"
)

// ResetDueToError reverses a change applied to a ref to the specified target
// ID. It is used to ensure a gittuf operation is atomic: if a gittuf operation
// fails, any changes made to the repository in refs/gittuf can be rolled back.
// Worktrees are not updated.
func (r *Repository) ResetDueToError(cause error, refName string, commitID Hash) error {
	if err := r.SetReference(refName, commitID); err != nil {
		return fmt.Errorf("unable to reset %s to %s, caused by following error: %w", refName, commitID.String(), cause)
	}
	return cause
}

func RemoteRef(refName, remoteName string) string {
	var remotePath string
	switch {
	case strings.HasPrefix(refName, BranchRefPrefix):
		// refs/heads/<path> -> refs/remotes/<remote>/<path>
		rest := strings.TrimPrefix(refName, BranchRefPrefix)
		remotePath = path.Join(RemoteRefPrefix, remoteName, rest)
	case strings.HasPrefix(refName, TagRefPrefix):
		// refs/tags/<path> -> refs/tags/<path>
		remotePath = refName
	default:
		// refs/<path> -> refs/remotes/<remote>/<path>
		rest := strings.TrimPrefix(refName, RefPrefix)
		remotePath = path.Join(RemoteRefPrefix, remoteName, rest)
	}

	return remotePath
}

// RestoreWorktree is a test helper to fix the worktree in tests where we need
// to operate in a checked out copy of the repository. This is primarily needed
// for support with older Git versions.
func (r *Repository) RestoreWorktree(t *testing.T) {
	t.Helper()

	worktree := r.gitDirPath
	if !r.IsBare() {
		worktree = strings.TrimSuffix(worktree, ".git") // TODO: this doesn't support detached git dir
	}
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(worktree); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(cwd) //nolint:errcheck

	if _, err := r.executor("restore", "--staged", ".").executeString(); err != nil {
		t.Fatal(err)
	}

	if _, err := r.executor("restore", ".").executeString(); err != nil {
		t.Fatal(err)
	}
}

func testNameToRefName(testName string) string {
	return BranchReferenceName(strings.ReplaceAll(testName, " ", "__"))
}
