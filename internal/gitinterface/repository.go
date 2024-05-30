// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"fmt"
	"io"
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

// Repository is a lightweight wrapper around a Git repository. It stores the
// location of the repository's GIT_DIR.
type Repository struct {
	gitDirPath string
	clock      clockwork.Clock
}

// GetGoGitRepository returns the go-git representation of a repository. We use
// this in certain signing and verifying workflows.
func (r *Repository) GetGoGitRepository() (*git.Repository, error) {
	return git.PlainOpenWithOptions(r.gitDirPath, &git.PlainOpenOptions{DetectDotGit: true})
}

// GetGitDir returns the GIT_DIR path for the repository.
func (r *Repository) GetGitDir() string {
	return r.gitDirPath
}

// LoadRepository returns a Repository instance using the current working
// directory. It also inspects the PATH to ensure Git is installed.
func LoadRepository() (*Repository, error) {
	_, err := exec.LookPath(binary)
	if err != nil {
		return nil, fmt.Errorf("unable to find Git binary, is Git installed?")
	}

	repo := &Repository{clock: clockwork.NewRealClock()}
	envVar := os.Getenv("GIT_DIR")
	if envVar != "" {
		repo.gitDirPath = envVar
		return repo, nil
	}

	gitDirPath, err := repo.executeGitCommandDirectString("rev-parse", "--git-dir")
	if err != nil {
		return nil, fmt.Errorf("unable to identify GIT_DIR: %w", err)
	}
	repo.gitDirPath = gitDirPath

	return repo, nil
}

// executeGitCommandString is a helper to execute the specified command in the
// repository. It automatically adds the explicit `--git-dir` parameter. It
// returns the stdout for successful execution as a string, with leading and
// trailing spaces removed.
func (r *Repository) executeGitCommandString(args ...string) (string, error) {
	args = append([]string{"--git-dir", r.gitDirPath}, args...)
	return r.executeGitCommandDirectString(args...)
}

// executeGitCommand is a helper to execute the specified command in the
// repository. It automatically adds the explicit `--git-dir` parameter.
func (r *Repository) executeGitCommand(args ...string) (io.Reader, io.Reader, error) { //nolint:unused
	args = append([]string{"--git-dir", r.gitDirPath}, args...)
	return r.executeGitCommandDirect(args...)
}

// executeGitCommandString is a helper to execute the specified command in the
// repository. It executes in the current directory without specifying the
// GIT_DIR explicitly. It returns the stdout for successful execution as a
// string, with leading and trailing spaces removed.
func (r *Repository) executeGitCommandDirectString(args ...string) (string, error) {
	stdOut, stdErr, err := r.executeGitCommandDirect(args...)
	if err != nil {
		stdErrContents, newErr := io.ReadAll(stdErr)
		if newErr != nil {
			return "", fmt.Errorf("unable to read stderr contents: %w; original err: %w", newErr, err)
		}
		return "", fmt.Errorf("%w when executing `git %s`: %s", err, strings.Join(args, " "), string(stdErrContents))
	}

	stdOutContents, err := io.ReadAll(stdOut)
	if err != nil {
		return "", fmt.Errorf("unable to read stdout contents: %w", err)
	}

	return strings.TrimSpace(string(stdOutContents)), nil
}

// executeGitCommandDirect is a helper to execute the specified command in the
// repository. It executes in the current directory without specifying the
// GIT_DIR explicitly.
func (r *Repository) executeGitCommandDirect(args ...string) (io.Reader, io.Reader, error) {
	return r.executeGitCommandDirectWithEnv(nil, args...)
}

// executeGitCommandWithEnvString is a helper to execute the specified command
// in the repository after setting the provided environment variables. Note that
// the command inherits os.Environ() first. This helper explicitly sets the
// GIT_DIR. It returns the stdout for successful execution as a string, with
// leading and trailing spaces removed.
func (r *Repository) executeGitCommandWithEnvString(env []string, args ...string) (string, error) {
	stdOut, stdErr, err := r.executeGitCommandWithEnv(env, args...)
	if err != nil {
		stdErrContents, newErr := io.ReadAll(stdErr)
		if newErr != nil {
			return "", fmt.Errorf("unable to read stderr contents: %w; original err: %w", newErr, err)
		}
		return "", fmt.Errorf("%w when executing `git %s`: %s", err, strings.Join(args, " "), string(stdErrContents))
	}

	stdOutContents, err := io.ReadAll(stdOut)
	if err != nil {
		return "", fmt.Errorf("unable to read stdout contents: %w", err)
	}

	return strings.TrimSpace(string(stdOutContents)), nil
}

// executeGitCommandWithEnv is a helper to execute the specified command in the
// repository after setting the provided environment variables. Note that the
// command inherits os.Environ() first. This helper explicitly sets the GIT_DIR.
func (r *Repository) executeGitCommandWithEnv(env []string, args ...string) (io.Reader, io.Reader, error) {
	args = append([]string{"--git-dir", r.gitDirPath}, args...)
	return r.executeGitCommandDirectWithEnv(env, args...)
}

// executeGitCommandDirectWithEnv is a helper to execute the specified command
// in the repository after setting the provided environment variables. Note that
// the command inherits os.Environ() first. This helper executes in the current
// directory without specifying the GIT_DIR explicitly.
func (r *Repository) executeGitCommandDirectWithEnv(env []string, args ...string) (io.Reader, io.Reader, error) {
	cmd := exec.Command(binary, args...)
	cmd.Env = append(os.Environ(), env...)

	var (
		stdOut bytes.Buffer
		stdErr bytes.Buffer
	)

	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	return &stdOut, &stdErr, cmd.Run()
}

// executeGitCommandWithStdInString is a helper to execute the specified command
// in the repository with the specified `stdIn`. It automatically adds the
// explicit `--git-dir` parameter.  It returns the stdout for successful
// execution as a string, with leading and trailing spaces removed.
func (r *Repository) executeGitCommandWithStdInString(stdIn *bytes.Buffer, args ...string) (string, error) {
	args = append([]string{"--git-dir", r.gitDirPath}, args...)
	stdOut, stdErr, err := r.executeGitCommandDirectWithStdIn(stdIn, args...)
	if err != nil {
		stdErrContents, newErr := io.ReadAll(stdErr)
		if newErr != nil {
			return "", fmt.Errorf("unable to read stderr contents: %w; original err: %w", newErr, err)
		}
		return "", fmt.Errorf("%w when executing `git %s`: %s", err, strings.Join(args, " "), string(stdErrContents))
	}

	stdOutContents, err := io.ReadAll(stdOut)
	if err != nil {
		return "", fmt.Errorf("unable to read stdout contents: %w", err)
	}

	return strings.TrimSpace(string(stdOutContents)), nil
}

// executeGitCommandDirectWithStdIn is a helper to execute the specified command
// in the repository with `stdIn` passed into the process stdin. It executes in
// the current directory without specifying the GIT_DIR explicitly.
func (r *Repository) executeGitCommandDirectWithStdIn(stdIn *bytes.Buffer, args ...string) (io.Reader, io.Reader, error) {
	cmd := exec.Command(binary, args...)

	var (
		stdOut bytes.Buffer
		stdErr bytes.Buffer
	)

	if stdIn != nil {
		cmd.Stdin = stdIn
	}
	cmd.Stdout = &stdOut
	cmd.Stderr = &stdErr

	return &stdOut, &stdErr, cmd.Run()
}
