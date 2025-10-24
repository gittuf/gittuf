// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"strings"
)

// GetGitConfig reads the applicable Git config for a repository and returns
// it. The "keys" for each config are normalized to lowercase.
func (r *Repository) GetGitConfig() (map[string]string, error) {
	stdOut, err := r.executor("config", "--get-regexp", `.*`).executeString()
	if err != nil {
		return nil, fmt.Errorf("unable to read Git config: %w", err)
	}

	config := map[string]string{}

	lines := strings.Split(strings.TrimSpace(stdOut), "\n")
	for _, line := range lines {
		split := strings.SplitN(line, " ", 2)
		if len(split) == 2 {
			config[strings.ToLower(split[0])] = split[1]
		} else if len(split) == 1 && split[0] == "gpg.format" {
			config[strings.ToLower(split[0])] = ""
		}
	}

	return config, nil
}

// SetGitConfig sets the specified key to the value locally for a repository.
func (r *Repository) SetGitConfig(key, value string) error {
	if _, err := r.executor("config", "--local", key, value).executeString(); err != nil {
		return fmt.Errorf("unable to set '%s' to '%s': %w", key, value, err)
	}

	return nil
}
