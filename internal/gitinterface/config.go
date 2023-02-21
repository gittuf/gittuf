package gitinterface

import (
	"bufio"
	"bytes"
	"fmt"
	"os/exec"
	"strings"
)

// GetConfig parses the user's Git config. It shells out to the Git binary
// because go-git has difficulty combining local, global, and system configs
// while maintaining all of their fields.
// See: https://github.com/go-git/go-git/issues/508
//
// Deprecated: This is a public function only as long as the `gittuf dev config`
// interface is around.
func GetConfig() (map[string]string, error) {
	config := map[string]string{}

	cmd := exec.Command("git", "config", "--get-regexp", `.*`)
	stdout := bytes.Buffer{}
	stderr := bytes.Buffer{}
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	if err := cmd.Run(); err != nil {
		return map[string]string{}, fmt.Errorf("%w: %s", err, stderr.String())
	}

	s := bufio.NewScanner(&stdout)
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
