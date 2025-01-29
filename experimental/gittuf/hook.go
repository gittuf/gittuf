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
	sandbox "github.com/gittuf/gittuf/internal/lua-sandbox"
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

// InvokeHook runs the hooks defined in the specified stage for the user defined
// by the supplied signer. A check is performed that the user holds the private
// key necessary for signing, to support the generation of attestations.
func (r *Repository) InvokeHook(ctx context.Context, stage string, signer sslibdsse.Signer, targetsRoleName string, attest bool) error {
	// TODO: Below is a check if we can sign and then invoke the appropriate lua
	// hook. We still need to add impl to sign attestation for invoking the
	// hook.
	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, false)
	if err != nil {
		return err
	}

	if attest {
		// This is to check if we can sign an attestation
		env, err := dsse.CreateEnvelope(targetsMetadata) // nolint:ineffassign
		if err != nil {
			return err
		}
		env, err = dsse.SignEnvelope(ctx, env, signer) // nolint:ineffassign,staticcheck
		if err != nil {
			return err
		}
	}

	applets, err := targetsMetadata.GetHooks(stage)
	if err != nil {
		return err
	}

	exitCodes := make([]int, 0, len(applets))
	for _, applet := range applets {
		exitCode, err := r.executeLua(stage, applet)
		exitCodes = append(exitCodes, exitCode) // nolint:staticcheck
		if err != nil {
			return err
		}
	}

	if attest {
		// TODO...
		slog.Debug(fmt.Sprintf("Signing hook attestation using '%s'...", keyID))
	}

	return err
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

func (r *Repository) executeLua(stage string, hook tuf.Applet) (int, error) {
	var hookContent string
	allowedExcecutables := hook.GetModules()
	hookHashes := hook.GetHashes()

	L, err := sandbox.NewLuaEnvironment(allowedExcecutables)
	if err != nil {
		return -1, err
	}

	gitRepo := r.GetGitRepository()
	parameters := parseHookParameters(stage, gitRepo)

	L.SetGlobal("hookParameters", lua.LString(parameters))

	// Get the hook content from hash
	hookFileContents, err := r.r.ReadBlob(hookHashes["sha1"])
	if err != nil {
		return -1, err
	}

	// Verify SHA256 hash
	sha256Hash := sha256.New()
	sha256Hash.Write(hookFileContents)
	calculatedSHA256 := sha256Hash.Sum(nil)
	if !bytes.Equal(calculatedSHA256, hookHashes["sha256"]) {
		return -1, fmt.Errorf("hook content SHA256 hash mismatch")
	}

	hookContent = string(hookFileContents)

	defer L.Close()

	if err := L.DoString(hookContent); err != nil {
		return -1, err
	}

	if exitcode := L.GetGlobal("hookExitCode").(lua.LNumber); exitcode != 0 {
		return int(exitcode), nil
	}

	return 0, nil
}

func parseHookParameters(stage string, gitRepo *gitinterface.Repository) string {
	parameters := make(map[string]interface{})

	switch stage {
	case "pre-commit":
		stagedFiles, _ := gitRepo.DiffStagedFiles()
		parameters["stagedFiles"] = stagedFiles

	case "commit-msg":
		if len(os.Args) > 1 {
			parameters["commitMessageFile"] = os.Args[1]
		}

	case "prepare-commit-msg":
		if len(os.Args) > 1 {
			parameters["commitMessageFile"] = os.Args[1]
		}
		if len(os.Args) > 2 {
			parameters["commitSource"] = os.Args[2]
		}
		if len(os.Args) > 3 {
			parameters["commitSHA"] = os.Args[3]
		}

	case "pre-push":
		if len(os.Args) > 1 {
			parameters["remoteName"] = os.Args[1]
		}
		if len(os.Args) > 2 {
			parameters["remoteURL"] = os.Args[2]
		}
		parameters["refs"] = parseStdinRefs()

	case "post-receive", "pre-receive":
		parameters["refs"] = parseStdinRefs()

	case "update":
		if len(os.Args) > 3 {
			parameters["refName"] = os.Args[1]
			parameters["oldRevision"] = os.Args[2]
			parameters["newRevision"] = os.Args[3]
		}

	case "post-merge":
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
