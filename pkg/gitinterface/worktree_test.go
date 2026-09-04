// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mustRunGit runs a Git command directly for test setup scenarios that have no
// Repository instance yet (e.g. `git init --separate-git-dir`) and fails the
// test with the command's output if it does not succeed. Ambient GIT_* environment
// variables are filtered out to keep the suite hermetic.
func mustRunGit(t *testing.T, args ...string) {
	t.Helper()

	cmd := exec.Command(binary, args...) //nolint:gosec
	env := []string{}
	for _, entry := range os.Environ() {
		key := strings.ToUpper(strings.SplitN(entry, "=", 2)[0])
		if key != "GIT_DIR" && key != "GIT_WORK_TREE" {
			env = append(env, entry)
		}
	}
	cmd.Env = env

	output, err := cmd.CombinedOutput()
	require.Nil(t, err, "git %v failed: %s", args, string(output))
}

func evalTestPath(t *testing.T, path string) string {
	t.Helper()

	resolved, err := filepath.EvalSymlinks(path)
	require.Nil(t, err)
	return resolved
}

func TestGetWorktree(t *testing.T) {
	t.Parallel()

	t.Run("standard layout", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false)

		worktree, err := repo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, tmpDir), worktree)
	})

	t.Run("standard layout, SHA-256", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, false, WithSHA256Format())

		worktree, err := repo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, tmpDir), worktree)
	})

	t.Run("loaded from subdirectory", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		CreateTestGitRepository(t, tmpDir, false)

		subDir := filepath.Join(tmpDir, "subdir")
		require.Nil(t, os.Mkdir(subDir, 0o755))

		repo, err := LoadRepository(subDir)
		require.Nil(t, err)

		worktree, err := repo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, tmpDir), worktree)
	})

	t.Run("detached git dir", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		worktreeDir := filepath.Join(tmpDir, "worktree")
		gitDirPath := filepath.Join(tmpDir, "storage.git")

		mustRunGit(t, "init", "--separate-git-dir", gitDirPath, worktreeDir)

		repo, err := LoadRepository(worktreeDir)
		require.Nil(t, err)

		// the legacy TrimSuffix approach resolves to <tmp>/storage, which does
		// not exist; GetWorktree must resolve to the actual worktree instead
		worktree, err := repo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, worktreeDir), worktree)
	})

	t.Run("explicit relative core.worktree", func(t *testing.T) {
		// Git interprets relative core.worktree values relative to the
		// GIT_DIR; submodules are configured this way. The repository is
		// constructed directly so that no load-time discovery takes
		// precedence and the core.worktree branch is exercised.
		t.Parallel()

		tmpDir := t.TempDir()
		worktreeDir := filepath.Join(tmpDir, "worktree")
		gitDirPath := filepath.Join(tmpDir, "modules", "repo.git")
		require.Nil(t, os.MkdirAll(filepath.Dir(gitDirPath), 0o755))

		mustRunGit(t, "init", "--separate-git-dir", gitDirPath, worktreeDir)

		repo := &Repository{gitDirPath: gitDirPath}
		_, _, err := repo.executor("config", "core.worktree", "../../worktree").execute()
		require.Nil(t, err)

		worktree, err := repo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, worktreeDir), worktree)
	})

	t.Run("stale core.worktree", func(t *testing.T) {
		// a core.worktree value pointing at a directory that is no longer
		// valid must be ignored rather than trusted
		t.Parallel()

		tmpDir := t.TempDir()
		worktreeDir := filepath.Join(tmpDir, "worktree")
		gitDirPath := filepath.Join(tmpDir, "storage.git")
		require.Nil(t, os.MkdirAll(filepath.Dir(gitDirPath), 0o755))

		mustRunGit(t, "init", "--separate-git-dir", gitDirPath, worktreeDir)

		repo, err := LoadRepository(worktreeDir)
		require.Nil(t, err)

		// point core.worktree at a nonexistent location
		_, _, err = repo.executor("config", "core.worktree", filepath.Join(tmpDir, "gone")).execute()
		require.Nil(t, err)

		worktree, err := repo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, worktreeDir), worktree)
	})

	t.Run("gitdir file pointing to another repository", func(t *testing.T) {
		// a $GIT_DIR/gitdir entry that points at a different repository must
		// not redirect worktree resolution
		t.Parallel()

		tmpDir := t.TempDir()
		gitDirPath := filepath.Join(tmpDir, ".git")
		mustRunGit(t, "init", tmpDir)

		otherRepoPath := filepath.Join(tmpDir, "other")
		mustRunGit(t, "init", otherRepoPath)

		repo, err := LoadRepository(tmpDir)
		require.Nil(t, err)
		require.Nil(t, os.WriteFile(filepath.Join(gitDirPath, "gitdir"), []byte(filepath.Join(otherRepoPath, ".git")+"\n"), 0o600))

		// the record is ignored and resolution falls back to the parent-of-.git
		// layout for this repository
		worktree, err := repo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, tmpDir), worktree)
	})

	t.Run("stale linked worktree record", func(t *testing.T) {
		// a $GIT_DIR/gitdir entry that points nowhere must not be trusted
		tmpDir := t.TempDir()
		gitDirPath := filepath.Join(tmpDir, ".git")
		mustRunGit(t, "init", tmpDir)

		repo := &Repository{gitDirPath: gitDirPath}
		require.Nil(t, os.WriteFile(filepath.Join(gitDirPath, "gitdir"), []byte("/does/not/exist/.git\n"), 0o600))

		// fall back to the standard layout instead of trusting the record
		worktree, err := repo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, tmpDir), worktree)
	})

	t.Run("linked worktree", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		mainRepoPath := filepath.Join(tmpDir, "main")
		linkedRepoPath := filepath.Join(tmpDir, "linked")
		repo := CreateTestGitRepository(t, mainRepoPath, false)

		// `git worktree add` requires at least one commit
		require.Nil(t, os.WriteFile(filepath.Join(mainRepoPath, "README.md"), []byte("test"), 0o600))
		_, err := repo.executor("add", ".").executeString()
		require.Nil(t, err)
		_, err = repo.executor("commit", "--no-gpg-sign", "-m", "initial commit").executeString()
		require.Nil(t, err)

		_, err = repo.executor("worktree", "add", linkedRepoPath, "-b", "feature").executeString()
		require.Nil(t, err)

		linkedRepo, err := LoadRepository(linkedRepoPath)
		require.Nil(t, err)

		// linked worktrees do not have a GIT_DIR ending in .git, so IsBare
		// must not classify them as bare
		assert.False(t, linkedRepo.IsBare())

		worktree, err := linkedRepo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, linkedRepoPath), worktree)

		// the main repository still resolves to its own worktree
		worktree, err = repo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, mainRepoPath), worktree)
	})

	t.Run("linked worktree, SHA-256", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		mainRepoPath := filepath.Join(tmpDir, "main")
		linkedRepoPath := filepath.Join(tmpDir, "linked")
		repo := CreateTestGitRepository(t, mainRepoPath, false, WithSHA256Format())

		require.Nil(t, os.WriteFile(filepath.Join(mainRepoPath, "README.md"), []byte("test"), 0o600))
		_, err := repo.executor("add", ".").executeString()
		require.Nil(t, err)
		_, err = repo.executor("commit", "--no-gpg-sign", "-m", "initial commit").executeString()
		require.Nil(t, err)

		_, err = repo.executor("worktree", "add", linkedRepoPath, "-b", "feature").executeString()
		require.Nil(t, err)

		// loading a linked worktree reads the repository config from the
		// common Git directory via commondir
		linkedRepo, err := LoadRepository(linkedRepoPath)
		require.Nil(t, err)
		assert.Equal(t, ObjectFormatSHA256, linkedRepo.GetObjectFormat())

		worktree, err := linkedRepo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, linkedRepoPath), worktree)
	})

	t.Run("status in linked worktree", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		mainRepoPath := filepath.Join(tmpDir, "main")
		linkedRepoPath := filepath.Join(tmpDir, "linked")
		repo := CreateTestGitRepository(t, mainRepoPath, false)

		require.Nil(t, os.WriteFile(filepath.Join(mainRepoPath, "README.md"), []byte("test"), 0o600))
		_, err := repo.executor("add", ".").executeString()
		require.Nil(t, err)
		_, err = repo.executor("commit", "--no-gpg-sign", "-m", "initial commit").executeString()
		require.Nil(t, err)

		_, err = repo.executor("worktree", "add", linkedRepoPath, "-b", "feature").executeString()
		require.Nil(t, err)

		untrackedFile := filepath.Join(linkedRepoPath, "untracked.txt")
		require.Nil(t, os.WriteFile(untrackedFile, []byte("test"), 0o600))

		linkedRepo, err := LoadRepository(linkedRepoPath)
		require.Nil(t, err)

		statuses, err := linkedRepo.Status()
		require.Nil(t, err)
		assert.Len(t, statuses, 1)
		status, has := statuses["untracked.txt"]
		require.True(t, has)
		assert.True(t, status.Untracked())
	})

	t.Run("bare repository", func(t *testing.T) {
		t.Parallel()

		tmpDir := t.TempDir()
		repo := CreateTestGitRepository(t, tmpDir, true)

		assert.True(t, repo.IsBare())

		_, err := repo.GetWorktree()
		assert.ErrorIs(t, err, ErrNoWorktree)

		// Status on a bare repository must surface a clear error rather than
		// running git inside an unrelated directory
		_, err = repo.Status()
		assert.ErrorIs(t, err, ErrNoWorktree)
	})

	t.Run("bare repository named with .git suffix", func(t *testing.T) {
		// the name-based heuristic misclassified these as non-bare
		t.Parallel()

		tmpDir := t.TempDir()
		bareDirPath := filepath.Join(tmpDir, "bare.git")
		repo := setupRepository(t, bareDirPath, true, ObjectFormatSHA1)

		assert.True(t, repo.IsBare())

		_, err := repo.GetWorktree()
		assert.ErrorIs(t, err, ErrNoWorktree)
	})

	t.Run("fallback without LoadRepository", func(t *testing.T) {
		// repositories constructed directly only carry their GIT_DIR, so
		// resolution falls back to Git's standard layout
		t.Parallel()

		tmpDir := t.TempDir()
		mustRunGit(t, "init", tmpDir)

		repo := &Repository{gitDirPath: filepath.Join(tmpDir, ".git")}

		worktree, err := repo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, tmpDir), worktree)
	})

	t.Run("loaded from subdirectory of linked worktree", func(t *testing.T) {
		// discovery must work when invoked from anywhere inside the linked
		// worktree, matching how git rev-parse resolves the toplevel
		t.Parallel()

		tmpDir := t.TempDir()
		mainRepoPath := filepath.Join(tmpDir, "main")
		linkedRepoPath := filepath.Join(tmpDir, "linked")
		repo := CreateTestGitRepository(t, mainRepoPath, false)

		require.Nil(t, os.WriteFile(filepath.Join(mainRepoPath, "README.md"), []byte("test"), 0o600))
		_, err := repo.executor("add", ".").executeString()
		require.Nil(t, err)
		_, err = repo.executor("commit", "--no-gpg-sign", "-m", "initial commit").executeString()
		require.Nil(t, err)

		_, err = repo.executor("worktree", "add", linkedRepoPath, "-b", "feature").executeString()
		require.Nil(t, err)

		nestedDir := filepath.Join(linkedRepoPath, "nested")
		require.Nil(t, os.Mkdir(nestedDir, 0o755))

		linkedRepo, err := LoadRepository(nestedDir)
		require.Nil(t, err)

		worktree, err := linkedRepo.GetWorktree()
		require.Nil(t, err)
		assert.Equal(t, evalTestPath(t, linkedRepoPath), worktree)
	})
}
