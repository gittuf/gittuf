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

func (l *logScanner) Buffer(buf []byte, max int) {
	l.scanner.Buffer(buf, max)
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
		fmt.Fprintf(logFile, fmtStr, messages...)
	}
}

func packetEncode(str string) []byte {
	return []byte(fmt.Sprintf("%04x%s", 4+len(str), str))
}

func getGittufWants(remoteTips map[string]string) ([]string, error) {
	wants := []string{}
	for remoteRef, tip := range remoteTips {
		cmd := exec.Command("git", "--git-dir", os.Getenv("GIT_DIR"), "rev-parse", remoteRef) //nolint:gosec
		output, err := cmd.Output()
		if err != nil {
			return nil, err
		}

		if string(bytes.TrimSpace(output)) != tip {
			wants = append(wants, tip)
		}
	}

	return wants, nil
}

func getSSHCommand() ([]string, error) {
	sshCmd := os.Getenv("GIT_SSH_COMMAND")
	if len(sshCmd) != 0 {
		return strings.Split(sshCmd, " "), nil
	}

	sshCmd = os.Getenv("GIT_SSH")
	if len(sshCmd) != 0 {
		return []string{sshCmd}, nil
	}

	cmd := exec.Command("git", "config", "--get-regexp", `.*`)
	stdOut, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("unable to read Git config: %w", err)
	}

	config := map[string]string{}

	lines := strings.Split(strings.TrimSpace(string(stdOut)), "\n")
	for _, line := range lines {
		split := strings.Split(line, " ")
		if len(split) < 2 {
			continue
		}
		config[strings.ToLower(split[0])] = strings.Join(split[1:], " ")
	}

	sshCmd, defined := config["core.sshcommand"]
	if defined {
		return strings.Split(sshCmd, " "), nil
	}

	return []string{"ssh"}, nil
}
