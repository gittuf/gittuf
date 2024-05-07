// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/jonboulle/clockwork"
)

const (
	binary           = "git"
	committerTimeKey = "GIT_COMMITTER_DATE"
	authorTimeKey    = "GIT_AUTHOR_DATE"
)

type Repository struct {
	gitDirPath string
	clock      clockwork.Clock
}

func (r *Repository) GetGoGitRepository() (*git.Repository, error) {
	return git.PlainOpenWithOptions(r.gitDirPath, &git.PlainOpenOptions{DetectDotGit: true})
}

func (r *Repository) GetGitDir() string {
	return r.gitDirPath
}

func LoadRepository() (*Repository, error) {
	repo := &Repository{clock: clockwork.NewRealClock()}
	envVar := os.Getenv("GIT_DIR")
	if envVar != "" {
		repo.gitDirPath = envVar
		return repo, nil
	}

	stdOut, stdErr, err := repo.executeGitCommandDirect("rev-parse", "--git-dir")
	if err != nil {
		return nil, fmt.Errorf("unable to identify GIT_DIR: %w: %s", err, stdErr)
	}
	repo.gitDirPath = strings.TrimSpace(stdOut)

	return repo, nil
}

func (r *Repository) executeGitCommand(args ...string) (string, string, error) {
	args = append([]string{"--git-dir", r.gitDirPath}, args...)
	return r.executeGitCommandDirect(args...)
}

func (r *Repository) executeGitCommandDirect(args ...string) (string, string, error) {
	cmd := exec.Command(binary, args...)

	var (
		stdOut bytes.Buffer
		stdErr bytes.Buffer
	)

	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	err := cmd.Run()
	stdOutString := stdOut.String() // sometimes we want the trailing new line (say when `cat-file -p` a blob, leaving it to the caller)
	stdErrString := strings.TrimSpace(stdErr.String())
	if err != nil {
		if stdErrString == "" {
			stdErrString = "error running `git " + strings.Join(args, " ") + "`"
		}
	}
	return stdOutString, stdErrString, err
}

func (r *Repository) executeGitCommandWithStdIn(stdInContents []byte, args ...string) (string, string, error) {
	args = append([]string{"--git-dir", r.gitDirPath}, args...)
	return r.executeGitCommandDirectWithStdIn(stdInContents, args...)
}

func (r *Repository) executeGitCommandDirectWithStdIn(stdInContents []byte, args ...string) (string, string, error) {
	cmd := exec.Command(binary, args...)

	var (
		stdOut bytes.Buffer
		stdErr bytes.Buffer
	)

	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	stdInWriter, err := cmd.StdinPipe()
	if err != nil {
		return "", "", fmt.Errorf("unable to create stdin writer: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return "", "", fmt.Errorf("error starting command: %w", err)
	}

	if _, err = stdInWriter.Write(stdInContents); err != nil {
		return "", "", fmt.Errorf("unable writing stdin contents: %w", err)
	}
	if err := stdInWriter.Close(); err != nil {
		return "", "", fmt.Errorf("unable to close stdin writer: %w", err)
	}

	err = cmd.Wait()
	stdOutString := stdOut.String() // sometimes we want the trailing new line (say when `cat-file -p` a blob, leaving it to the caller)
	stdErrString := strings.TrimSpace(stdErr.String())
	if err != nil {
		if stdErrString == "" {
			stdErrString = "error running `git " + strings.Join(args, " ") + "`"
		}
	}
	return stdOutString, stdErrString, err
}
