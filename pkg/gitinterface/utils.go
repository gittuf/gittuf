// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"path"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/internal/common/testutils"
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

	if _, err := r.executor("restore", "--staged", ".").executeString(); err != nil {
		t.Fatal(err)
	}

	if _, err := r.executor("restore", ".").executeString(); err != nil {
		t.Fatal(err)
	}
}

// FixKeyPermissionsForWindows is a test helper to fix file permissions on Windows.
// It uses icacls to restrict access to the current user.
func FixKeyPermissionsForWindows(t testing.TB, path string) {
	t.Helper()
	testutils.FixKeyPermissionsForWindows(t, path)
}

func testNameToRefName(testName string) string {
	return BranchReferenceName(strings.ReplaceAll(testName, " ", "__"))
}
