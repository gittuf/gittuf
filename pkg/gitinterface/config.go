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
// used as per https://git-scm.com/book/en/v2/Customizing-Git-Git-Configuration.
// The "keys" for each config are normalized to lowercase.
func GetGitConfig(repo string) (map[string]string, error) {
	var stdOut string

	if repo != "" {
		output, err := exec.Command("git", "-C", repo, "config", "--get-regexp", `.*`).Output()
		if err != nil {
			return nil, fmt.Errorf("unable to read Git config: %w", err)
		}
		stdOut = string(output)
	} else {
		output, err := exec.Command("git", "config", "--get-regexp", `.*`).Output()
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
