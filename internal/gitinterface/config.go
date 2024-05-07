// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

var (
	getGitConfigFromCommand = execGitConfig // variable used to override in tests
	getGitConfig            = getRealGitConfig
)

// GetConfig parses the user's Git config. It shells out to the Git binary
// because go-git has difficulty combining local, global, and system configs
// while maintaining all of their fields.
// See: https://github.com/go-git/go-git/issues/508
func getConfig() (map[string]string, error) {
	configReader, err := getGitConfigFromCommand()
	if err != nil {
		return nil, err
	}

	config := map[string]string{}

	s := bufio.NewScanner(configReader)
	for s.Scan() {
		raw := s.Text()
		data := strings.Split(raw, " ")
		if len(data) < 2 {
			continue
		}
		config[data[0]] = strings.Join(data[1:], " ")
	}

	return config, nil
}

func execGitConfig() (io.Reader, error) {
	cmd := exec.Command("git", "config", "--get-regexp", `.*`)
	stdout := &bytes.Buffer{}
	stderr := &bytes.Buffer{}
	cmd.Stdout = stdout
	cmd.Stderr = stderr
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("%w: %s", err, stderr.String())
	}

	return stdout, nil
}

func getRealGitConfig(repo *git.Repository) (*config.Config, error) {
	return repo.ConfigScoped(config.GlobalScope)
}

// Possible we don't need this if we rely on git binary to identify config for
// signing, committer identity, etc.
func (r *Repository) GetGitConfig() (map[string]string, error) {
	stdOut, stdErr, err := r.executeGitCommand("config", "--get-regexp", `.*`)
	if err != nil {
		return nil, fmt.Errorf("unable to read Git config: %s", stdErr)
	}

	config := map[string]string{}

	lines := strings.Split(strings.TrimSpace(stdOut), "\n")
	for _, line := range lines {
		split := strings.Split(line, " ")
		if len(split) < 2 {
			continue
		}
		config[strings.ToLower(split[0])] = strings.Join(split[1:], " ")
	}

	return config, nil
}

func (r *Repository) SetGitConfig(key, value string) error {
	if _, stdErr, err := r.executeGitCommand("config", "--local", key, value); err != nil {
		return fmt.Errorf("unable to set '%s' to '%s': %s", key, value, stdErr)
	}

	return nil
}
