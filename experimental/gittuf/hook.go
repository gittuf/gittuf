// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/luasandbox"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
)

type ErrHookExists struct {
	HookType HookType
}

func (e *ErrHookExists) Error() string {
	return fmt.Sprintf("hook '%s' already exists", e.HookType)
}

type HookType string

var (
	ErrNoHooksFoundForPrincipal = errors.New("no hooks found for the specified principal")
)

var HookPrePush = HookType("pre-push")

// InvokeHook runs the hooks defined in the specified stage for the user defined
// by principalID. Upon successful completion of all hooks for the stage for the
// user, the map of hook names to exit codes is returned.
// TODO: Add attestations workflow
func (r *Repository) InvokeHook(ctx context.Context, stage tuf.HookStage, principalID, targetsRoleName string, parameters ...string) (map[string]int, error) {
	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyRef)
	if err != nil {
		return nil, err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return nil, err
	}
	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, false)
	if err != nil {
		return nil, err
	}

	allHooks, err := rootMetadata.GetHooks(stage)
	if err != nil {
		return nil, err
	}

	var selectedHooks []tuf.Hook
	var selectedPrincipal tuf.Principal
	var found = false

	// Read the principals from targetsMetadata and attempt to find a match for
	// the specified principal to determine which hooks to run.
	for _, principal := range targetsMetadata.GetPrincipals() {
		if principal.ID() == principalID {
			selectedPrincipal = principal
			found = true
			break
		}
	}

	// Couldn't match the key up to a principal, abort
	if !found {
		return nil, tuf.ErrPrincipalNotFound
	}

	// Now, read all hooks for the specified stage and find which ones we need
	// to run
	for _, hook := range allHooks {
		principalIDs := hook.GetPrincipalIDs()
		if principalIDs.Has(selectedPrincipal.ID()) {
			selectedHooks = append(selectedHooks, hook)
		}
	}

	exitCodes := make(map[string]int, len(selectedHooks))
	for _, hook := range selectedHooks {
		exitCode, err := r.executeHook(ctx, hook, parameters...)
		if err != nil {
			return nil, err
		}
		exitCodes[hook.ID()] = exitCode
	}

	return exitCodes, nil
}

// UpdateHook updates a git hook in the repository's .git/hooks folder.
// Existing hook files are not overwritten, unless force flag is set.
func (r *Repository) UpdateHook(hookType HookType, content []byte, force bool) error {
	// TODO: rely on go-git to find .git folder, once
	// https://github.com/go-git/go-git/issues/977 is available.
	// Note, until then gittuf does not support separate git dir.

	slog.Debug("Adding gittuf hooks...")

	gitDir := r.r.GetGitDir()

	hookFolder := filepath.Join(gitDir, "hooks")
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

func (r *Repository) executeHook(ctx context.Context, hook tuf.Hook, parameters ...string) (int, error) {
	var hookContents string
	hookHashes := hook.GetHashes()

	environment, err := luasandbox.NewLuaEnvironment(ctx, r.r)
	if err != nil {
		return -1, err
	}
	defer environment.Cleanup()

	// Load the hook contents from the repository
	hookHash, err := gitinterface.NewHash(hookHashes[gitinterface.GitBlobHashName])
	if err != nil {
		return -1, err
	}
	hookFileContents, err := r.r.ReadBlob(hookHash)
	if err != nil {
		return -1, err
	}

	hookContents = string(hookFileContents)

	exitCode, err := environment.RunScript(hookContents, parameters...)
	if err != nil {
		return -1, err
	}

	return exitCode, nil
}
