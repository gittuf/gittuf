// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// execCmd is the low-level command runner used by runGit and runCmd.
// It returns trimmed stdout on success, or a wrapped error with stderr on failure.
func execCmd(dir string, name string, args ...string) (string, error) {
	cmd := exec.Command(name, args...)
	cmd.Dir = dir

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	out := strings.TrimSpace(stdout.String())
	if err != nil {
		return out, fmt.Errorf("%s %s failed: %w\nstderr: %s",
			name, strings.Join(args, " "), err, strings.TrimSpace(stderr.String()))
	}
	return out, nil
}

// runCmd is an alias for execCmd, used when the command is not git.
func runCmd(dir string, name string, args ...string) (string, error) {
	return execCmd(dir, name, args...)
}
