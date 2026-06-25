// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v6"
	"github.com/jonboulle/clockwork"
)

const (
	binary           = "git"
	committerTimeKey = "GIT_COMMITTER_DATE"
	authorTimeKey    = "GIT_AUTHOR_DATE"
)

var ErrRepositoryPathNotSpecified = errors.New("repository path not specified")

// Repository is a lightweight wrapper around a Git repository. It stores the
// location of the repository's GIT_DIR.
type Repository struct {
	gitDirPath   string
	worktreePath string
	clock        clockwork.Clock
}

// GetGoGitRepository returns the go-git representation of a repository. We use
// this in certain signing and verifying workflows.
func (r *Repository) GetGoGitRepository() (*git.Repository, error) {
	return git.PlainOpenWithOptions(r.gitDirPath, &git.PlainOpenOptions{DetectDotGit: !r.IsBare()})
}

// GetGitDir returns the GIT_DIR path for the repository.
func (r *Repository) GetGitDir() string {
	return r.gitDirPath
}

// IsBare returns true if the repository is a bare repository.
func (r *Repository) IsBare() bool {
	// TODO: this may not work when the repo is cloned with GIT_DIR set
	// elsewhere. We don't support this at the moment, so it's probably okay?
	return !strings.HasSuffix(r.gitDirPath, ".git")
}

// LoadRepository returns a Repository instance using the current working
// directory. It also inspects the PATH to ensure Git is installed.
func LoadRepository(repositoryPath string) (*Repository, error) {
	slog.Debug("Looking for Git binary in PATH...")
	_, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("unable to find Git binary, is Git installed?")
	}
	if repositoryPath == "" {
		return nil, ErrRepositoryPathNotSpecified
	}

	repo := &Repository{clock: clockwork.NewRealClock(), worktreePath: repositoryPath}

	slog.Debug("Identifying git directory for repository...")
	stdOut, stdErr, err := repo.executor("rev-parse", "--git-dir").withoutGitDir().execute()
	stdOutContents, _ := io.ReadAll(stdOut)
	stdErrContents, _ := io.ReadAll(stdErr)

	if err != nil {
		return nil, fmt.Errorf("unable to identify git directory for repository: %w: %s", err, strings.TrimSpace(string(stdErrContents)))
	}

	gitDir := strings.TrimSpace(string(stdOutContents))
	var absPath string
	if filepath.IsAbs(gitDir) {
		absPath, err = filepath.Abs(gitDir)
	} else {
		absPath, err = filepath.Abs(filepath.Join(repositoryPath, gitDir))
	}
	if err != nil {
		return nil, err
	}

	absRepoPath, err := filepath.Abs(repositoryPath)
	if err != nil {
		return nil, err
	}

	cleanAbsPath := filepath.Clean(absPath)
	cleanAbsRepoPath := filepath.Clean(absRepoPath)

	lowerAbsPath := strings.ToLower(cleanAbsPath)
	lowerAbsRepoPath := strings.ToLower(cleanAbsRepoPath)

	isBare := lowerAbsPath == lowerAbsRepoPath
	if !isBare {
		worktreePath := filepath.Dir(cleanAbsPath)
		lowerWorktreePath := strings.ToLower(worktreePath)
		if lowerAbsRepoPath != lowerWorktreePath && !strings.HasPrefix(lowerAbsRepoPath, lowerWorktreePath+string(filepath.Separator)) {
			return nil, fmt.Errorf("unable to identify git directory for repository: detected git dir %s is outside repository path %s", absPath, absRepoPath)
		}
	}

	slog.Debug(fmt.Sprintf("Setting git directory for repository to '%s'...", cleanAbsPath))
	repo.gitDirPath = cleanAbsPath

	return repo, nil
}

// executor is a lightweight wrapper around exec.Cmd to run Git commands. It
// accepts the arguments to the `git` binary, but the binary itself must not be
// specified.
type executor struct {
	r           *Repository
	args        []string
	env         []string
	stdIn       io.Reader
	unsetGitDir bool
}

// executor initializes a new executor instance to run a Git command with the
// specified arguments.
func (r *Repository) executor(args ...string) *executor {
	return &executor{r: r, args: args, env: os.Environ()}
}

// withEnv adds the specified environment variables. Each environment variable
// must be specified in the form of `key=value`.
func (e *executor) withEnv(env ...string) *executor {
	e.env = append(e.env, env...)
	return e
}

// withoutGitDir ensures the executor doesn't auto-set the --git-dir flag to the
// executed command.
func (e *executor) withoutGitDir() *executor {
	e.unsetGitDir = true
	return e
}

// executeString runs the constructed Git command and returns the contents of
// stdout.  Leading and trailing spaces and newlines are removed. This function
// should be used almost every time; the only exception is when the output is
// desired without any processing such as the removal of space characters.
func (e *executor) executeString() (string, error) {
	stdOut, stdErr, err := e.execute()
	if err != nil {
		stdErrContents, newErr := io.ReadAll(stdErr)
		if newErr != nil {
			return "", fmt.Errorf("unable to read stderr contents: %w; original err: %w", newErr, err)
		}
		return "", fmt.Errorf("%w when executing `git %s`: %s", err, strings.Join(e.args, " "), string(stdErrContents))
	}

	stdOutContents, err := io.ReadAll(stdOut)
	if err != nil {
		return "", fmt.Errorf("unable to read stdout contents: %w", err)
	}

	return strings.TrimSpace(string(stdOutContents)), nil
}

// execute runs the constructed Git command and returns the raw stdout and
// stderr contents. It adds the `--git-dir` argument if the repository has a
// path set.
func (e *executor) execute() (io.Reader, io.Reader, error) {
	if e.r.gitDirPath != "" && !e.unsetGitDir {
		e.args = append([]string{"--git-dir", e.r.gitDirPath}, e.args...)
	}
	cmd := exec.Command(binary, e.args...) //nolint:gosec
	cmd.Env = e.env
	cmd.Env = append(cmd.Env, "LC_ALL=C") // force git to the C (and thus english) locale
	if e.r.worktreePath != "" {
		cmd.Dir = e.r.worktreePath
	}

	var (
		stdOut bytes.Buffer
		stdErr bytes.Buffer
	)

	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	if e.stdIn != nil {
		cmd.Stdin = e.stdIn
	}

	err := cmd.Run()

	return &stdOut, &stdErr, err
}
