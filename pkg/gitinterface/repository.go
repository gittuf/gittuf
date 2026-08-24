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
	gogitconfig "github.com/go-git/go-git/v6/config"
	"github.com/jonboulle/clockwork"
)

const (
	binary           = "git"
	committerTimeKey = "GIT_COMMITTER_DATE"
	authorTimeKey    = "GIT_AUTHOR_DATE"
)

var (
	ErrRepositoryPathNotSpecified    = errors.New("repository path not specified")
	ErrUnknownObjectFormat           = errors.New("unknown object format")
	ErrCompatObjectFormatUnsupported = errors.New("gittuf does not support repositories with extensions.compatObjectFormat enabled")
	ErrNoWorktree                    = errors.New("repository has no worktree")
)

// Repository is a lightweight wrapper around a Git repository. It stores the
// location of the repository's GIT_DIR. If the repository has a worktree and
// it was discovered while loading the repository, its location is also
// recorded.
type Repository struct {
	gitDirPath   string
	worktreePath string
	objectFormat ObjectFormat
	clock        clockwork.Clock
}

// GetObjectFormat returns the hash algorithm the repository uses for its object
// IDs.
func (r *Repository) GetObjectFormat() ObjectFormat {
	return r.objectFormat
}

// readObjectFormat queries Git for the repository's object format (hash
// algorithm).
func (r *Repository) readObjectFormat() (ObjectFormat, error) {
	stdOut, err := r.executor("rev-parse", "--show-object-format").executeString()
	if err != nil {
		return "", fmt.Errorf("unable to read object format: %w", err)
	}

	switch format := ObjectFormat(stdOut); format {
	case ObjectFormatSHA1, ObjectFormatSHA256:
		return format, nil
	default:
		return "", fmt.Errorf("%w: %s", ErrUnknownObjectFormat, stdOut)
	}
}

// ensureNoCompatObjectFormat returns an error if the repository is in dual
// hash interop mode (extensions.compatObjectFormat). In that mode Git
// maintains both SHA-1 and SHA-256 representations of every object and stores
// additional compat signatures under headers gittuf does not process, so
// signing and verification results would be unreliable. The config file is
// read directly (without invoking Git) because Git builds without compat
// support refuse to open such repositories at all.
func (r *Repository) ensureNoCompatObjectFormat() error {
	configPath := filepath.Join(r.gitDirPath, "config")

	// Linked worktrees do not have their own config file: configuration lives
	// in the repository's common Git directory, whose location is recorded in
	// $GIT_DIR/commondir.
	if contents, err := os.ReadFile(filepath.Join(r.gitDirPath, "commondir")); err == nil {
		commonDirPath := strings.TrimSpace(string(contents))
		if commonDirPath != "" {
			if !filepath.IsAbs(commonDirPath) {
				commonDirPath = filepath.Join(r.gitDirPath, commonDirPath)
			}
			commonConfigPath := filepath.Join(commonDirPath, "config")
			if fileInfo, err := os.Stat(commonConfigPath); err == nil && fileInfo.Mode().IsRegular() {
				configPath = commonConfigPath
			}
		}
	}

	configFile, err := os.Open(configPath)
	if err != nil {
		return fmt.Errorf("unable to read repository config: %w", err)
	}
	defer configFile.Close() //nolint:errcheck

	gitConfig, err := gogitconfig.ReadConfig(configFile)
	if err != nil {
		return fmt.Errorf("unable to parse repository config: %w", err)
	}
	compatFormat := gitConfig.Raw.Section("extensions").Options.Get("compatObjectFormat")
	if compatFormat != "" {
		return fmt.Errorf("%w: compat object format is set to '%s'", ErrCompatObjectFormatUnsupported, compatFormat)
	}

	return nil
}

func findGitDirPath(startPath string) (string, string, bool, error) {
	currentPath, err := filepath.Abs(startPath)
	if err != nil {
		return "", "", false, err
	}

	for {
		gitDirPath := filepath.Join(currentPath, ".git")
		if fileInfo, err := os.Stat(gitDirPath); err == nil {
			if fileInfo.IsDir() {
				return gitDirPath, currentPath, true, nil
			}

			resolvedGitDirPath, err := readGitDirFile(gitDirPath, currentPath)
			if err != nil {
				return "", "", false, err
			}
			return resolvedGitDirPath, currentPath, true, nil
		} else if !os.IsNotExist(err) {
			return "", "", false, err
		}

		if isBareGitDir(currentPath) {
			return currentPath, "", true, nil
		}

		parentPath := filepath.Dir(currentPath)
		if parentPath == currentPath {
			return "", "", false, nil
		}
		currentPath = parentPath
	}
}

func readGitDirFile(gitDirFilePath, worktreePath string) (string, error) {
	contents, err := os.ReadFile(gitDirFilePath)
	if err != nil {
		return "", err
	}

	gitDirPath, has := strings.CutPrefix(strings.TrimSpace(string(contents)), "gitdir:")
	if !has {
		return "", fmt.Errorf("invalid gitdir file: %s", gitDirFilePath)
	}
	gitDirPath = strings.TrimSpace(gitDirPath)
	if !filepath.IsAbs(gitDirPath) {
		gitDirPath = filepath.Join(worktreePath, gitDirPath)
	}

	return filepath.Abs(gitDirPath)
}

func isBareGitDir(path string) bool {
	if fileInfo, err := os.Stat(filepath.Join(path, "config")); err != nil || fileInfo.IsDir() {
		return false
	}
	if fileInfo, err := os.Stat(filepath.Join(path, "HEAD")); err != nil || fileInfo.IsDir() {
		return false
	}
	return true
}

// resolvePath returns the absolute, symlink-resolved form of the specified
// path. If the path cannot be resolved, the absolute path is returned.
func resolvePath(path string) string {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	resolvedPath, err := filepath.EvalSymlinks(absPath)
	if err != nil {
		return absPath
	}
	return resolvedPath
}

// GetGoGitRepository returns the go-git representation of a repository. We use
// this in certain signing and verifying workflows.
func (r *Repository) GetGoGitRepository() (*git.Repository, error) {
	// gitDirPath is already the resolved git directory (set via
	// `git rev-parse --git-dir` in LoadRepository), so DetectDotGit must be
	// false: with it true, go-git looks for a .git entry inside this path,
	// which a bare repository does not have, and returns ErrRepositoryNotExists.
	return git.PlainOpenWithOptions(r.gitDirPath, &git.PlainOpenOptions{DetectDotGit: false})
}

// GetGitDir returns the GIT_DIR path for the repository.
func (r *Repository) GetGitDir() string {
	return r.gitDirPath
}

// IsBare returns true if the repository is a bare repository, i.e. it has no
// worktree. Bareness is determined by Git itself; if Git cannot be consulted,
// we fall back to checking for the sentinel files Git writes on bare
// repositories.
func (r *Repository) IsBare() bool {
	stdOut, err := r.executor("rev-parse", "--is-bare-repository").executeString()
	if err != nil {
		return isBareGitDir(r.gitDirPath)
	}
	return stdOut == "true"
}

// GetWorktree returns the absolute path of the repository's worktree, i.e.
// the directory containing the checked out files. It returns an error wrapping
// ErrNoWorktree if the repository is bare or if the worktree cannot be
// determined.
//
// Resolves:
//  1. linked worktrees record the location of their `.git` file in
//     `$GIT_DIR/gitdir`; the link is verified in both directions
//  2. the worktree discovered while loading the repository, which is the only
//     reliable source for repositories with a detached GIT_DIR
//  3. `core.worktree` from the repository-local configuration, set explicitly
//     for repositories like submodules
//  4. otherwise, a `$GIT_DIR` named `.git` implies its parent directory is
//     the worktree
//
// Callers can test for ErrNoWorktree with errors.Is.
func (r *Repository) GetWorktree() (string, error) {
	if r.IsBare() {
		return "", ErrNoWorktree
	}

	if contents, err := os.ReadFile(filepath.Join(r.gitDirPath, "gitdir")); err == nil {
		gitDirFilePath := strings.TrimSpace(string(contents))
		if gitDirFilePath != "" {
			worktree := filepath.Dir(gitDirFilePath)
			if !filepath.IsAbs(worktree) {
				worktree = filepath.Join(r.gitDirPath, worktree)
			}
			worktree = resolvePath(worktree)
			// Git records the location of the worktree's `.git` entry here.
			// Verify the link in the other direction too: the `.git` file it
			// points at must reference this GIT_DIR, so a stale pointer that
			// now belongs to a different repository is rejected.
			if isUsableWorktree(r.gitDirPath, worktree) {
				if gitDirBelongsTo(worktree, r.gitDirPath) {
					return worktree, nil
				}
			}
		}
	}

	if r.worktreePath != "" {
		return resolvePath(r.worktreePath), nil
	}

	if worktree, err := r.executor("config", "--local", "--get", "core.worktree").executeString(); err == nil && worktree != "" {
		if !filepath.IsAbs(worktree) {
			// Git interprets relative core.worktree values relative to the
			// GIT_DIR.
			worktree = filepath.Join(r.gitDirPath, worktree)
		}
		worktree = resolvePath(worktree)
		if isUsableWorktree(r.gitDirPath, worktree) {
			return worktree, nil
		}
	}

	if filepath.Base(r.gitDirPath) == ".git" {
		return resolvePath(filepath.Dir(r.gitDirPath)), nil
	}

	return "", fmt.Errorf("%w: unable to determine worktree for '%s'", ErrNoWorktree, r.gitDirPath)
}

// gitDirBelongsTo returns true if the `.git` entry at the specified worktree
// resolves to the supplied GIT_DIR. A conventional `.git` directory matches
// only when its resolved path is the GIT_DIR; a `.git` file matches when its
// `gitdir:` pointer resolves to the GIT_DIR.
func gitDirBelongsTo(worktree, gitDirPath string) bool {
	resolvedGitDir := resolvePath(gitDirPath)
	gitEntryPath := filepath.Join(worktree, ".git")
	fileInfo, err := os.Stat(gitEntryPath)
	if err != nil {
		return false
	}
	if fileInfo.IsDir() {
		return filepath.Clean(resolvePath(gitEntryPath)) == filepath.Clean(resolvedGitDir)
	}
	gitFileContents, err := os.ReadFile(gitEntryPath)
	if err != nil {
		return false
	}
	link, has := strings.CutPrefix(strings.TrimSpace(string(gitFileContents)), "gitdir:")
	if !has {
		return false
	}
	link = strings.TrimSpace(link)
	if !filepath.IsAbs(link) {
		link = filepath.Join(worktree, link)
	}
	return filepath.Clean(resolvePath(link)) == filepath.Clean(resolvedGitDir)
}

// isUsableWorktree returns true if the candidate path is an existing directory
// other than the repository's Git directory.
func isUsableWorktree(gitDirPath, candidate string) bool {
	fileInfo, err := os.Stat(candidate)
	return err == nil &&
		fileInfo.IsDir() &&
		filepath.Clean(resolvePath(candidate)) != filepath.Clean(resolvePath(gitDirPath))
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

	repo := &Repository{clock: clockwork.NewRealClock()}

	gitDirPath, worktreePath, has, err := findGitDirPath(repositoryPath)
	if err != nil {
		return nil, err
	}
	if has {
		repo.gitDirPath = gitDirPath
		repo.worktreePath = worktreePath
		if err := repo.ensureNoCompatObjectFormat(); err != nil {
			return nil, err
		}
	}

	slog.Debug("Identifying git directory for repository...")
	stdOut, stdErr, err := repo.executor("rev-parse", "--absolute-git-dir").withoutGitDir().withDir(repositoryPath).execute()
	if err != nil {
		errContents, newErr := io.ReadAll(stdErr)
		if newErr != nil {
			return nil, fmt.Errorf("unable to read original err '%w' when loading repository: %w", err, newErr)
		}
		return nil, fmt.Errorf("unable to identify git directory for repository: %w: %s", err, strings.TrimSpace(string(errContents)))
	}

	stdOutContents, err := io.ReadAll(stdOut)
	if err != nil {
		return nil, fmt.Errorf("unable to identify git directory for repository: %w", err)
	}

	absPath, err := filepath.EvalSymlinks(strings.TrimSpace(string(stdOutContents)))
	if err != nil {
		return nil, err
	}
	slog.Debug(fmt.Sprintf("Setting git directory for repository to '%s'...", absPath))
	repo.gitDirPath = absPath

	// Capture the worktree using the same invocation context that resolved
	// the Git directory so that the two remain consistent even when
	// environment variables like GIT_DIR influence discovery. Note that
	// `--show-toplevel` must run without an explicit `--git-dir`: with one,
	// Git regards the working directory itself as the top level. For bare
	// repositories this fails and no worktree is recorded.
	stdOut, stdErr, err = repo.executor("rev-parse", "--show-toplevel").withoutGitDir().withDir(repositoryPath).execute()
	if err == nil {
		if worktreeContents, readErr := io.ReadAll(stdOut); readErr == nil {
			if worktree := strings.TrimSpace(string(worktreeContents)); worktree != "" {
				slog.Debug(fmt.Sprintf("Setting worktree for repository to '%s'...", worktree))
				repo.worktreePath = worktree
			}
		}
	} else {
		errContents, readErr := io.ReadAll(stdErr)
		if readErr == nil {
			slog.Debug(fmt.Sprintf("Repository '%s' does not have a worktree: %s", absPath, strings.TrimSpace(string(errContents))))
		}
	}

	objectFormat, err := repo.readObjectFormat()
	if err != nil {
		return nil, err
	}
	repo.objectFormat = objectFormat

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
	dir         string
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

// withDir runs the command with the given working directory instead of the
// process's. Use this for worktree-relative commands (status, restore) so the
// process-global os.Chdir is never touched.
func (e *executor) withDir(dir string) *executor {
	e.dir = dir
	return e
}

// withStdIn sets the contents of stdin to be passed in to the command.
func (e *executor) withStdIn(stdIn *bytes.Buffer) *executor {
	e.stdIn = stdIn
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
	cmd.Env = append(cmd.Env, "LC_ALL=C")                 // force git to the C (and thus english) locale
	cmd.Env = append(cmd.Env, "GIT_NO_REPLACE_OBJECTS=1") // ignore refs/replace/ so verification reads reflect the true objects, matching the replace-blind go-git reads
	if e.dir != "" {
		cmd.Dir = e.dir
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
