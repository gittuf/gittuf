// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/common/set"
)

type logWriteCloser struct {
	name        string
	writeCloser io.WriteCloser
}

func (l *logWriteCloser) Write(p []byte) (int, error) {
	prefix := fmt.Sprintf("writing to %s", l.name)

	trimmed := bytes.TrimSpace(p)
	if len(trimmed) != len(p) {
		prefix += " (space trimmed)"
	}

	prefix += ":"

	log(prefix, string(trimmed))
	return l.writeCloser.Write(p)
}

func (l *logWriteCloser) Close() error {
	return l.writeCloser.Close()
}

type logReadCloser struct {
	name       string
	readCloser io.ReadCloser
}

func (l *logReadCloser) Read(p []byte) (int, error) {
	n, err := l.readCloser.Read(p)

	prefix := fmt.Sprintf("reading from %s", l.name)

	trimmed := bytes.TrimSpace(p)
	if len(trimmed) != len(p) {
		prefix += " (space trimmed)"
	}

	prefix += ":"

	log(prefix, string(trimmed))
	return n, err
}

func (l *logReadCloser) Close() error {
	return l.readCloser.Close()
}

type logScanner struct {
	name    string
	scanner *bufio.Scanner
}

func (l *logScanner) Buffer(buf []byte, maxN int) {
	l.scanner.Buffer(buf, maxN)
}

func (l *logScanner) Bytes() []byte {
	b := l.scanner.Bytes()

	prefix := fmt.Sprintf("scanner %s returned", l.name)

	trimmed := bytes.TrimSpace(b)
	if len(trimmed) != len(b) {
		prefix += " (space trimmed)"
	}

	prefix += ":"

	log(prefix, b, string(trimmed))
	return b
}

func (l *logScanner) Err() error {
	return l.scanner.Err()
}

func (l *logScanner) Scan() bool {
	return l.scanner.Scan()
}

func (l *logScanner) Split(split bufio.SplitFunc) {
	l.scanner.Split(split)
}

func (l *logScanner) Text() string {
	t := l.scanner.Text()

	prefix := fmt.Sprintf("scanner %s returned", l.name)

	trimmed := strings.TrimSpace(t)
	if len(trimmed) != len(t) {
		prefix += " (space trimmed)"
	}

	prefix += ":"

	log(prefix, trimmed)
	return t
}

func log(messages ...any) {
	if len(messages) == 0 {
		return
	}
	fmtStr := "%v"
	for i := 1; i < len(messages); i++ {
		fmtStr += " %v"
	}
	fmtStr += "\n"
	if logFile != nil {
		fmt.Fprintf(logFile, fmtStr, messages...) //nolint:gosec
	}
}

func packetEncode(str string) []byte {
	return []byte(fmt.Sprintf("%04x%s", 4+len(str), str))
}

func getGittufWantsAndHaves(repo *gittuf.Repository, remoteTips map[string]string) (map[string]string, []string, error) {
	wants := map[string]string{}
	currentTips := set.NewSet[string]()
	for remoteRef, tip := range remoteTips {
		currentTip, err := repo.GetGitRepository().GetReference(remoteRef)
		if err != nil {
			return nil, nil, err
		}

		if currentTip.String() != tip {
			wants[remoteRef] = tip
		}
		currentTips.Add(currentTip.String())
	}

	return wants, currentTips.Contents(), nil
}

func getSSHCommand(repo *gittuf.Repository) ([]string, error) {
	sshCmd := os.Getenv("GIT_SSH_COMMAND")
	if len(sshCmd) != 0 {
		return strings.Split(sshCmd, " "), nil
	}

	sshCmd = os.Getenv("GIT_SSH")
	if len(sshCmd) != 0 {
		return []string{sshCmd}, nil
	}

	config, err := repo.GetGitRepository().GetGitConfig()
	if err != nil {
		return nil, err
	}

	sshCmd, defined := config["core.sshcommand"]
	if defined {
		return strings.Split(sshCmd, " "), nil
	}

	return []string{"ssh"}, nil
}

func testSSH(sshCmd []string, host string) error {
	command := append(sshCmd, "-T", host)           //nolint:gocritic
	cmd := exec.Command(command[0], command[1:]...) //nolint:gosec
	if output, err := cmd.CombinedOutput(); err != nil {
		if cmd.ProcessState.ExitCode() == 255 {
			// with GitHub, we see exit code 1 while with GitLab and BitBucket,
			// we see exit code 0
			return fmt.Errorf("%s: %s", err.Error(), bytes.TrimSpace(output))
		}
	}
	return nil
}
