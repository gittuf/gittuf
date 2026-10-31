// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gittuf/gittuf/pkg/gitstore"
)

type ErrHookExists struct {
	HookType HookType
}

func (e *ErrHookExists) Error() string {
	return fmt.Sprintf("hook '%s' already exists", e.HookType)
}

type HookType string

var (
	HookPrePush      = HookType("pre-push")
	ErrHooksDisabled = errors.New("cannot install hook: Git hooks are disabled by core.hooksPath")
)

// UpdateGitHook updates a git hook in the repository's hooks folder.
// If core.hooksPath is configured, the hook is placed in that directory.
// Otherwise, it is placed in the repository's .git/hooks directory.
// Existing hook files are not overwritten, unless force flag is set.
func (r *Repository) UpdateGitHook(hookType HookType, content []byte, force bool) error {
	// TODO: rely on go-git to find .git folder, once
	// https://github.com/go-git/go-git/issues/977 is available.
	// Note, until then gittuf does not support separate git dir.

	slog.Debug("Adding gittuf hooks...")

	gitDir := r.r.GetGitDir()

	hookFolder := filepath.Join(gitDir, "hooks")
	hooksPath, ok, err := r.r.LookupConfig(gitstore.ConfigCoreHooksPath)
	if err != nil {
		return err
	}
	if ok && strings.TrimSpace(hooksPath) != "" {
		hooksPath = strings.TrimSpace(hooksPath)
		if hooksPath == "/dev/null" || hooksPath == os.DevNull || strings.EqualFold(hooksPath, "nul") {
			return ErrHooksDisabled
		}
		if filepath.IsAbs(hooksPath) {
			hookFolder = hooksPath
		} else {
			worktree := gitDir
			if !r.r.IsBare() {
				worktree = filepath.Dir(gitDir)
			}
			hookFolder = filepath.Join(worktree, hooksPath)
		}
	}
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
