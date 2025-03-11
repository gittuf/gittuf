// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/luasandbox"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
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

var HookPrePush = HookType("pre-push")

// InvokeHook runs the hooks defined in the specified stage for the user defined
// by the supplied signer. A check is performed that the user holds the private
// key necessary for signing, to support the generation of attestations. Upon
// successful completion of all hooks for the stage for the user, the map of
// hook names to exit codes is returned.
func (r *Repository) InvokeHook(ctx context.Context, stage tuf.HookStage, signer sslibdsse.Signer, targetsRoleName string, attest bool) (map[string]int, error) {
	keyID, err := signer.KeyID()
	if err != nil {
		return nil, err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
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

	hooks, err := rootMetadata.GetHooks(stage)
	if err != nil {
		return nil, err
	}

	if attest {
		// This is to check if we can sign an attestation
		env, err := dsse.CreateEnvelope(rootMetadata) // nolint:ineffassign
		if err != nil {
			return nil, err
		}
		_, err = dsse.SignEnvelope(ctx, env, signer) // nolint:ineffassign,staticcheck
		if err != nil {
			return nil, err
		}
	}

	applets := []tuf.Hook{}

	// Read the principals from targetsMetadata and attempt to find a match for
	// the specified principal to determine which hooks to run.
	for _, hook := range hooks {
		principals := targetsMetadata.GetPrincipals()
		principalIDs := hook.GetPrincipalIDs()
		for _, principalID := range principalIDs.Contents() {
			principal := principals[principalID]
			keys := principal.Keys()
			for _, key := range keys {
				if key.KeyID == keyID {
					applets = append(applets, hook)
				}
			}
		}
	}

	exitCodes := make(map[string]int, len(applets))
	for _, hook := range applets {
		exitCode, err := r.executeLua(ctx, stage, hook)
		if err != nil {
			return nil, err
		}
		exitCodes[hook.ID()] = exitCode // nolint:staticcheck
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

func (r *Repository) executeLua(ctx context.Context, stage tuf.HookStage, hook tuf.Hook) (int, error) {
	var hookContents string
	hookHashes := hook.GetHashes()

	L, err := luasandbox.NewLuaEnvironment(ctx)
	if err != nil {
		return -1, err
	}

	gitRepo := r.GetGitRepository()
	parameters := parseHookParameters(stage, gitRepo)

	L.SetGlobal("hookParameters", lua.LString(parameters))

	// Get the hook content from hash
	hookSHA1Hash, err := gitinterface.NewHash(hookHashes[gitinterface.GitBlobHashName])
	if err != nil {
		return 0, err
	}
	hookFileContents, err := r.r.ReadBlob(hookSHA1Hash)
	if err != nil {
		return -1, err
	}

	// Verify SHA256 hash
	sha256Hash := sha256.New()
	sha256Hash.Write(hookFileContents)
	calculatedSHA256 := sha256Hash.Sum(nil)
	if !bytes.Equal(calculatedSHA256, []byte(hookHashes[gitinterface.SHA256HashName])) {
		return -1, fmt.Errorf("hook content SHA256 hash mismatch")
	}

	hookContents = string(hookFileContents)

	defer L.Close()

	if err := L.DoString(hookContents); err != nil {
		return -1, err
	}

	if exitcode := L.GetGlobal("hookExitCode").(lua.LNumber); exitcode != 0 {
		return int(exitcode), nil
	}

	return 0, nil
}

func parseHookParameters(stage tuf.HookStage, gitRepo *gitinterface.Repository) string {
	parameters := make(map[string]any)

	switch stage {
	case tuf.HookStagePreCommit:
		stagedFiles, _ := gitRepo.DiffStagedFiles()
		parameters["stagedFiles"] = stagedFiles

	case tuf.HookStageCommitMsg:
		if len(os.Args) > 1 {
			parameters["commitMessageFile"] = os.Args[1]
		}

	case tuf.HookStagePrepareCommitMsg:
		if len(os.Args) > 1 {
			parameters["commitMessageFile"] = os.Args[1]
		}
		if len(os.Args) > 2 {
			parameters["commitSource"] = os.Args[2]
		}
		if len(os.Args) > 3 {
			parameters["commitSHA"] = os.Args[3]
		}

	case tuf.HookStagePrePush:
		if len(os.Args) > 1 {
			parameters["remoteName"] = os.Args[1]
		}
		if len(os.Args) > 2 {
			parameters["remoteURL"] = os.Args[2]
		}
		parameters["refs"] = parseStdinRefs()

	case tuf.HookStagePostReceive, tuf.HookStagePreReceive:
		parameters["refs"] = parseStdinRefs()

	case tuf.HookStageUpdate:
		if len(os.Args) > 3 {
			parameters["refName"] = os.Args[1]
			parameters["oldRevision"] = os.Args[2]
			parameters["newRevision"] = os.Args[3]
		}

	case tuf.HookStagePostMerge:
		isSquashMerge := gitRepo.IsSquashMerge()
		parameters["isSquashMerge"] = isSquashMerge

	default:
		parameters["error"] = fmt.Sprintf("Unsupported stage: %s", stage)
	}

	jsonParameters, err := json.Marshal(parameters)
	if err != nil {
		return "{}"
	}

	return string(jsonParameters)
}

func parseStdinRefs() []map[string]string {
	var refs []map[string]string

	scanner := bufio.NewScanner(os.Stdin)
	for scanner.Scan() {
		line := scanner.Text()
		parts := strings.Fields(line)
		if len(parts) == 3 {
			ref := map[string]string{
				"oldRevision": parts[0],
				"newRevision": parts[1],
				"refName":     parts[2],
			}
			refs = append(refs, ref)
		}
	}
	return refs
}
