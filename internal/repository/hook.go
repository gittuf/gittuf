// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
)

type ErrHookExists struct {
	HookType HookType
}

func (e *ErrHookExists) Error() string {
	return fmt.Sprintf("hook '%s' already exists", e.HookType)
}

type HookType string

var HookPrePush = HookType("pre-push")

// UpdateHook updates a git hook in the repositorie's .git/hooks folder.
// Existing hook files are not overwritten, unless force flag is set.
func (r *Repository) UpdateHook(hookType HookType, content []byte, force bool) error {
	// TODO: rely on go-git to find .git folder, once
	// https://github.com/go-git/go-git/issues/977 is available.
	// Note, until then gittuf does not support separate git dir.

	slog.Debug("Adding gittuf hooks...")

	slog.Debug("Loading repository worktree...")
	tree, err := r.r.Worktree()
	if err != nil {
		return fmt.Errorf("reading worktree: %w", err)
	}
	if tree == nil {
		return fmt.Errorf("worktree is nil, can't update hooks")
	}

	repoRoot := tree.Filesystem.Root()
	hookFolder := filepath.Join(repoRoot, ".git", "hooks")
	if err := os.MkdirAll(hookFolder, 0o750); err != nil {
		return fmt.Errorf("making sure folder exist: %w", err)
	}

	hookFile := filepath.Join(hookFolder, string(hookType))
	hookExists, err := doesFileExist(hookFile)
	if err != nil {
		return fmt.Errorf("checking if hookFile '%s' exists: %w", hookFile, err)
	}

	if hookExists && !force {
		return &ErrHookExists{
			HookType: hookType,
		}
	}

	slog.Debug("Writing hooks...")
	if err := os.WriteFile(hookFile, content, 0o700); err != nil { // nolint:gosec
		return fmt.Errorf("writing %s hook: %w", hookType, err)
	}
	return nil
}

func doesFileExist(path string) (bool, error) {
	_, err := os.Stat(path)
	if err != nil {
		if errors.Is(err, fs.ErrNotExist) {
			return false, nil
		}
		return false, err
	}

	return true, nil
}
