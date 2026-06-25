// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package testutils

import (
	"os"
	"os/exec"
	"runtime"
	"testing"
)

// FixKeyPermissionsForWindows is a test helper to fix file permissions on Windows.
// It uses icacls to restrict access to the current user.
func FixKeyPermissionsForWindows(t testing.TB, path string) {
	t.Helper()
	if runtime.GOOS == "windows" {
		output, err := exec.Command("icacls", path, "/inheritance:r", "/grant:r", os.Getenv("USERNAME")+":F").CombinedOutput() //nolint:gosec
		if err != nil {
			t.Fatalf("failed to fix key permissions on %q with icacls: %v\noutput: %s", path, err, output)
		}
	}
}
