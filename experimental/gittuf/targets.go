// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"crypto/sha256"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
)

var ErrInvalidPolicyName = errors.New("invalid rule or policy file name, cannot be 'root'")

// InitializeTargets is the interface for the user to create the specified
// policy file.
func (r *Repository) InitializeTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, signCommit bool) error {
	if targetsRoleName == policy.RootRoleName {
		return ErrInvalidPolicyName
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}
	if state.HasTargetsRole(targetsRoleName) {
		return ErrCannotReinitialize
	}

	// TODO: verify is role can be signed using the presented key. This requires
	// the user to pass in the delegating role as well as we do not want to
	// assume which role is the delegating role (diamond delegations are legal).
	// See: https://github.com/gittuf/gittuf/issues/246.

	slog.Debug("Creating initial rule file...")
	targetsMetadata := policy.InitializeTargetsMetadata()

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing initial rule file using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		if state.DelegationEnvelopes == nil {
			state.DelegationEnvelopes = map[string]*sslibdsse.Envelope{}
		}
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Initialize policy '%s'", targetsRoleName)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}

// AddDelegation is the interface for the user to add a new rule to gittuf
// policy.
func (r *Repository) AddDelegation(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, ruleName string, authorizedPrincipalIDs, rulePatterns []string, threshold int, signCommit bool) error {
	if ruleName == policy.RootRoleName {
		return ErrInvalidPolicyName
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	slog.Debug("Checking if rule with same name exists...")
	if state.HasRuleName(ruleName) {
		return tuf.ErrDuplicatedRuleName
	}

	slog.Debug("Loading current rule file...")
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	// TODO: verify if role can be signed using the presented key. This requires
	// the user to pass in the delegating role as well as we do not want to
	// assume which role is the delegating role (diamond delegations are legal).
	// See: https://github.com/gittuf/gittuf/issues/246.

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, false)
	if err != nil {
		return err
	}

	slog.Debug("Adding rule to rule file...")
	if err := targetsMetadata.AddRule(ruleName, authorizedPrincipalIDs, rulePatterns, threshold); err != nil {
		return err
	}

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing updated rule file using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Add rule '%s' to policy '%s'", ruleName, targetsRoleName)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}

// UpdateDelegation is the interface for the user to update a rule to gittuf
// policy.
func (r *Repository) UpdateDelegation(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, ruleName string, authorizedPrincipalIDs, rulePatterns []string, threshold int, signCommit bool) error {
	if ruleName == policy.RootRoleName {
		return ErrInvalidPolicyName
	}

	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	slog.Debug("Loading current rule file...")
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	// TODO: verify if role can be signed using the presented key. This requires
	// the user to pass in the delegating role as well as we do not want to
	// assume which role is the delegating role (diamond delegations are legal).
	// See: https://github.com/gittuf/gittuf/issues/246.

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, false)
	if err != nil {
		return err
	}

	slog.Debug("Updating rule in rule file...")
	if err := targetsMetadata.UpdateRule(ruleName, authorizedPrincipalIDs, rulePatterns, threshold); err != nil {
		return err
	}

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing updated rule file using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Update rule '%s' in policy '%s'", ruleName, targetsRoleName)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}

// ReorderDelegations is the interface for the user to reorder rules in gittuf
// policy.
func (r *Repository) ReorderDelegations(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, ruleNames []string, signCommit bool) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return nil
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	slog.Debug("Loading current rule file...")
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, false)
	if err != nil {
		return err
	}

	slog.Debug("Reordering rules in rule file...")
	if err := targetsMetadata.ReorderRules(ruleNames); err != nil {
		return err
	}

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing updated rule file using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Reorder rules in policy '%s'", targetsRoleName)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}

// RemoveDelegation is the interface for a user to remove a rule from gittuf
// policy.
func (r *Repository) RemoveDelegation(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, ruleName string, signCommit bool) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	slog.Debug("Loading current rule file...")
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	// TODO: verify if role can be signed using the presented key. This requires
	// the user to pass in the delegating role as well as we do not want to
	// assume which role is the delegating role (diamond delegations are legal).
	// See: https://github.com/gittuf/gittuf/issues/246.

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, false)
	if err != nil {
		return err
	}

	slog.Debug("Removing rule from rule file...")
	if err := targetsMetadata.RemoveRule(ruleName); err != nil {
		return err
	}

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing updated rule file using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Remove rule '%s' from policy '%s'", ruleName, targetsRoleName)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}

// AddPrincipalToTargets is the interface for a user to add a trusted principal
// to gittuf rule file metadata.
func (r *Repository) AddPrincipalToTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, authorizedPrincipals []tuf.Principal, signCommit bool) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	// TODO: verify is role can be signed using the presented key. This requires
	// the user to pass in the delegating role as well as we do not want to
	// assume which role is the delegating role (diamond delegations are legal).
	// See: https://github.com/gittuf/gittuf/issues/246.

	principalIDs := ""
	for _, principal := range authorizedPrincipals {
		principalIDs += fmt.Sprintf("\n%s", principal.ID())
	}

	slog.Debug("Loading current rule file...")
	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, false)
	if err != nil {
		return err
	}

	for _, principal := range authorizedPrincipals {
		slog.Debug(fmt.Sprintf("Adding principal '%s' to rule file...", strings.TrimSpace(principal.ID())))
		if err := targetsMetadata.AddPrincipal(principal); err != nil {
			return err
		}
	}

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing updated rule file using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Add principals to policy '%s'\n%s", targetsRoleName, principalIDs)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}

// SignTargets adds a signature to specified Targets role's envelope. Note that
// the metadata itself is not modified, so its version remains the same.
func (r *Repository) SignTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, signCommit bool) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	var env *sslibdsse.Envelope
	if targetsRoleName == policy.TargetsRoleName {
		env = state.TargetsEnvelope
	} else {
		env = state.DelegationEnvelopes[targetsRoleName]
	}

	slog.Debug(fmt.Sprintf("Signing rule file using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Add signature from key '%s' to policy '%s'", keyID, targetsRoleName)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}

// AddHook defines the workflow for adding a file to be executed as a hook. It
// writes the hook file, populates all fields in the hooks metadata associated
// with this file and commits it to the policy.
func (r *Repository) AddHook(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName, hookName, filePath, stage, environment string, modules, principalIDs []string, signCommit bool) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	if hookName == "" {
		hookName = filepath.Base(filePath)
	}

	slog.Debug("Loading current rule file...")
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, false)
	if err != nil {
		return err
	}

	err = state.LoadHooksIntoState(targetsMetadata)
	if err != nil {
		return nil
	}

	slog.Debug("Checking if hook with same name exists...")
	switch stage {
	case "pre-commit":
		if state.PreCommitHooks[hookName] != nil {
			return tuf.ErrDuplicatedHookName
		}
	case "pre-push":
		if state.PrePushHooks[hookName] != nil {
			return tuf.ErrDuplicatedHookName
		}
	default:
		return tuf.ErrInvalidHookStage
	}

	slog.Debug("Reading hook from filesystem...")
	hookFile, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer hookFile.Close() // nolint:errcheck

	hookFileContents, err := io.ReadAll(hookFile)
	if err != nil {
		return err
	}

	var hashes = make(map[string]gitinterface.Hash, 2)

	blobID, err := r.r.WriteBlob(hookFileContents)
	if err != nil {
		return err
	}
	//TODO: hash agility
	hashes["sha1"] = blobID

	sha256Hash := sha256.New()
	sha256Hash.Write(hookFileContents)
	sha256HashSum := sha256Hash.Sum(nil)
	hashes["sha256"] = sha256HashSum

	slog.Debug("Adding hook to rule file...")
	if err := targetsMetadata.AddHook(stage, hookName, environment, hashes, modules, principalIDs); err != nil {
		return err
	}

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return err
	}

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	switch stage {
	case "pre-commit":
		state.PreCommitHooks[hookName] = blobID
	case "pre-push":
		state.PrePushHooks[hookName] = blobID
	default:
		return tuf.ErrInvalidHookStage
	}

	commitMessage := fmt.Sprintf("Add hook '%s' to policy '%s'", hookName, targetsRoleName)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}

// RemoveHook defines the workflow for removing a hook defined in gittuf policy.
func (r *Repository) RemoveHook(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName, hookName, stage string, signCommit bool) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	slog.Debug("Loading current rule file...")
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	targetsMetadata, err := state.GetTargetsMetadata(policy.TargetsRoleName, false)
	if err != nil {
		return err
	}

	slog.Debug("Checking if hook with name exists...")
	hooks, err := targetsMetadata.GetHooks(stage)
	if err != nil {
		return err
	}
	if hooks[hookName] == nil {
		return tuf.ErrHookNotFound
	}

	slog.Debug("Removing hook from rule file...")
	if err := targetsMetadata.RemoveHook(stage, hookName); err != nil {
		return err
	}

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return err
	}

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	switch stage {
	case "pre-commit":
		delete(state.PreCommitHooks, hookName)
	case "pre-push":
		delete(state.PrePushHooks, hookName)
	default:
		return tuf.ErrInvalidHookStage
	}

	commitMessage := fmt.Sprintf("Remove hook '%s' from policy '%s'", hookName, targetsRoleName)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}
