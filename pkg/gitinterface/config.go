// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"os/exec"
	"strings"
)

// GetGitConfig reads the applicable Git config for a repository and returns it.
// If the supplied repository is nil, the system or global Git config will be
// used as per
// https://github.com/ayuxsh009/gittuf/tree/fix/gpg-program-publickey-path. The
// "keys" for each config are normalized to lowercase.
func GetGitConfig(repo *Repository) (map[string]string, error) {
	var stdOut string
	var err error

	if repo != nil {
		stdOut, err = repo.executor("config", "--get-regexp", `.*`).executeString()
		if err != nil {
			return nil, fmt.Errorf("unable to read Git config: %w", err)
		}
	} else {
		output, err := exec.Command("config", "--global", "--get-regexp", `.*`).Output()
		if err != nil {
			return nil, fmt.Errorf("unable to read Git config: %w", err)
		}
		stdOut = string(output)
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
