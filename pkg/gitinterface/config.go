// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/pkg/gitstore"
)

// LookupConfig returns the value of a single Git config setting. ok is false
// when the key is not set. A key that is set to an empty value returns "" with
// ok true, matching `git config --get`, which exits 0 for a set-but-empty key
// and 1 for an unset one.
func (r *Repository) LookupConfig(key gitstore.ConfigKey) (string, bool, error) {
	stdOut, stdErr, err := r.executor("config", "--get", string(key)).execute()
	if err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) && exitErr.ExitCode() == 1 {
			return "", false, nil
		}
		stdErrContents, readErr := io.ReadAll(stdErr)
		if readErr != nil {
			return "", false, fmt.Errorf("unable to read Git config key '%s': %w", key, err)
		}
		return "", false, fmt.Errorf("unable to read Git config key '%s': %w: %s", key, err, strings.TrimSpace(string(stdErrContents)))
	}

	value, err := io.ReadAll(stdOut)
	if err != nil {
		return "", false, fmt.Errorf("unable to read Git config value for '%s': %w", key, err)
	}

	return strings.TrimSpace(string(value)), true, nil
}

// SetGitConfig sets the specified key to the value locally for a repository.
func (r *Repository) SetGitConfig(key, value string) error {
	if _, err := r.executor("config", "--local", key, value).executeString(); err != nil {
		return fmt.Errorf("unable to set '%s' to '%s': %w", key, value, err)
	}

	return nil
}
