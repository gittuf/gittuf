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
	"strings"

	hookopts "github.com/gittuf/gittuf/experimental/gittuf/options/hooks"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/luasandbox"
	luasandboxopts "github.com/gittuf/gittuf/internal/luasandbox/options/luasandbox"
	"github.com/gittuf/gittuf/internal/policy"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	lua "github.com/yuin/gopher-lua"
)

type ErrHookExists struct {
	HookType HookType
}

func (e *ErrHookExists) Error() string {
	return fmt.Sprintf("hook '%s' already exists", e.HookType)
}

type HookType string

const (
	pushedRefLocalRef   = lua.LString("localRef")
	pushedRefRemoteRef  = lua.LString("remoteRef")
	pushedRefLocalHash  = lua.LString("localHash")
	pushedRefRemoteHash = lua.LString("remoteHash")
)

var (
	ErrNoHooksFoundForPrincipal = errors.New("no hooks found for the specified principal")
)

var HookPrePush = HookType("pre-push")

// InvokeHooksForStage runs the hooks defined in the specified stage for the
// user defined by principalID. Upon successful completion of all hooks for the
// stage for the user, the map of hook names to exit codes is returned.  TODO:
// Add attestations workflow
func (r *Repository) InvokeHooksForStage(ctx context.Context, signer sslibdsse.Signer, stage tuf.HookStage, opts ...hookopts.Option) (map[string]int, error) {
	options := &hookopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	if signer == nil {
		return nil, sslibdsse.ErrNoSigners
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return nil, err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyRef)
	if err != nil {
		return nil, err
	}

	rootMetadata, err := state.GetRootMetadata(false)
	if err != nil {
		return nil, err
	}

	allHooks, err := rootMetadata.GetHooks(stage)
	if err != nil {
		return nil, err
	}

	// Read the principals from targetsMetadata and attempt to find a match for
	// the specified principal to determine which hooks to run.
	var selectedPrincipal tuf.Principal
	for _, principal := range state.GetAllPrincipals() {
		for _, key := range principal.Keys() {
			if key.KeyID == keyID {
				selectedPrincipal = principal
				break
			}
		}
	}

	// Couldn't match the key up to a principal, abort
	if selectedPrincipal == nil {
		return nil, tuf.ErrPrincipalNotFound
	}

	// Now, read all hooks for the specified stage and find which ones we need
	// to run
	selectedHooks := []tuf.Hook{}
	for _, hook := range allHooks {
		principalIDs := hook.GetPrincipalIDs()
		if principalIDs.Has(selectedPrincipal.ID()) {
			selectedHooks = append(selectedHooks, hook)
		}
	}

	if len(selectedHooks) == 0 {
		return nil, ErrNoHooksFoundForPrincipal
	}

	// Determine what parameters must be supplied based on the hook stage
	var luaParameters lua.LTable

	// At the moment, the only stage that we support that requires parameters is
	// the pre-push stage.
	if stage == tuf.HookStagePrePush {
		// https://git-scm.com/docs/githooks#_pre_push
		// For pre-push hooks, we supply:
		// * remoteName
		// * remoteURL
		// * localRef
		// * localHash
		// * remoteRef
		// * remoteHash

		// TODO (adityasaky): I wonder if we want to move these into separate
		// constructors / helpers as we add more stages...

		// Validate that all the required values have been passed in
		if err := options.PrePush.Validate(); err != nil {
			return nil, err
		}

		luaParameters.RawSet(lua.LString("remoteName"), lua.LString(options.PrePush.RemoteName))
		luaParameters.RawSet(lua.LString("remoteURL"), lua.LString(options.PrePush.RemoteURL))

		remoteObjects := make(map[string][]gitinterface.Hash, len(options.PrePush.RefSpecs))

		for _, refSpec := range options.PrePush.RefSpecs {
			splitRefSpec := strings.Split(refSpec, ":")
			localRef, remoteRef := splitRefSpec[0], splitRefSpec[1]
			remoteTrackerRef := gitinterface.RemoteRef(remoteRef, options.PrePush.RemoteName)

			var remoteHash gitinterface.Hash
			err = r.r.FetchRefSpec(options.PrePush.RemoteName, []string{fmt.Sprintf("%s:%s", remoteRef, remoteTrackerRef)})
			if err != nil {
				// This likely means the remote doesn't have the specified ref.
				// In this case, provide a zero hash as per original Git
				// behavior.
				remoteHash = gitinterface.ZeroHash
			} else {
				remoteHash, err = r.r.GetReference(remoteTrackerRef)
				if err != nil {
					return nil, err
				}
			}

			localHash, err := r.r.GetReference(localRef)
			if err != nil {
				return nil, err
			}

			remoteObjects[refSpec] = []gitinterface.Hash{localHash, remoteHash}
		}

		i := 0
		for refSpec, hashes := range remoteObjects {
			splitRefSpec := strings.Split(refSpec, ":")
			localRef, remoteRef := splitRefSpec[0], splitRefSpec[1]

			// The best type for supplying information about pushed refs that
			// gopher-lua supports without too much work is the LTable. We
			// create a table for each ref that is being pushed, with an
			// equivalent format in Go: []PushRef{{localRef: <>, remoteRef: <>,
			// localHash: <>, remoteHash: <>}}
			pushedRef := lua.LTable{}
			pushedRef.RawSet(pushedRefLocalRef, lua.LString(localRef))
			pushedRef.RawSet(pushedRefRemoteRef, lua.LString(remoteRef))
			pushedRef.RawSet(pushedRefLocalHash, lua.LString(hashes[0]))
			pushedRef.RawSet(pushedRefRemoteHash, lua.LString(hashes[1]))
			luaParameters.Insert(i, &pushedRef)
			i++
		}
	}

	exitCodes := make(map[string]int, len(selectedHooks))
	for _, hook := range selectedHooks {
		exitCode, err := r.executeHook(ctx, hook, luaParameters)
		if err != nil {
			return nil, err
		}
		exitCodes[hook.ID()] = exitCode
	}

	return exitCodes, nil
}

// UpdateHook updates a git hook in the repository's .git/hooks folder.
// Existing hook files are not overwritten, unless force flag is set.
func (r *Repository) UpdateGitHook(hookType HookType, content []byte, force bool) error {
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

func (r *Repository) executeHook(ctx context.Context, hook tuf.Hook, parameters lua.LTable) (int, error) {
	var hookContents string
	hookBlobID := hook.GetBlobID()

	environment, err := luasandbox.NewLuaEnvironment(ctx, r.r, luasandboxopts.WithLuaTimeout(hook.GetTimeout()))
	if err != nil {
		return -1, err
	}
	defer environment.Cleanup()

	// Load the hook contents from the repository
	hookFileContents, err := r.r.ReadBlob(hookBlobID)
	if err != nil {
		return -1, err
	}

	hookContents = string(hookFileContents)

	exitCode, err := environment.RunScript(hookContents, parameters)
	if err != nil {
		return -1, err
	}

	return exitCode, nil
}
