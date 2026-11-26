// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/policy"
	policyopts "github.com/gittuf/gittuf/internal/policy/options/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
)

var ErrInvalidPolicyName = errors.New("invalid rule or policy file name, cannot be 'root'")

// InitializeTargets is the interface for the user to create the specified
// policy file.
func (r *Repository) InitializeTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, signCommit bool, opts ...trustpolicyopts.Option) error {
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

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
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

	slog.Debug("Signing initial rule file...")
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.Metadata.TargetsEnvelope = env
	} else {
		if state.Metadata.DelegationEnvelopes == nil {
			state.Metadata.DelegationEnvelopes = map[string]*sslibdsse.Envelope{}
		}
		state.Metadata.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Initialize policy '%s'", targetsRoleName)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddDelegation is the interface for the user to add a new rule to gittuf
// policy.
func (r *Repository) AddDelegation(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, ruleName string, authorizedPrincipalIDs, rulePatterns []string, threshold int, signCommit bool, opts ...trustpolicyopts.Option) error {
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

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
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

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, true)
	if err != nil {
		return err
	}

	slog.Debug("Adding rule to rule file...")
	if err := targetsMetadata.AddRule(ruleName, authorizedPrincipalIDs, rulePatterns, threshold); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add rule '%s' to policy '%s'", ruleName, targetsRoleName)
	return r.updateTargetsMetadata(ctx, state, signer, targetsRoleName, targetsMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// UpdateDelegation is the interface for the user to update a rule to gittuf
// policy.
func (r *Repository) UpdateDelegation(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, ruleName string, authorizedPrincipalIDs, rulePatterns []string, threshold int, signCommit bool, opts ...trustpolicyopts.Option) error {
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

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
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

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, true)
	if err != nil {
		return err
	}

	slog.Debug("Updating rule in rule file...")
	if err := targetsMetadata.UpdateRule(ruleName, authorizedPrincipalIDs, rulePatterns, threshold); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Update rule '%s' in policy '%s'", ruleName, targetsRoleName)
	return r.updateTargetsMetadata(ctx, state, signer, targetsRoleName, targetsMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// ReorderDelegations is the interface for the user to reorder rules in gittuf
// policy.
func (r *Repository) ReorderDelegations(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, ruleNames []string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}

	slog.Debug("Loading current rule file...")
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, true)
	if err != nil {
		return err
	}

	slog.Debug("Reordering rules in rule file...")
	if err := targetsMetadata.ReorderRules(ruleNames); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Reorder rules in policy '%s'", targetsRoleName)
	return r.updateTargetsMetadata(ctx, state, signer, targetsRoleName, targetsMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// RemoveDelegation is the interface for a user to remove a rule from gittuf
// policy.
func (r *Repository) RemoveDelegation(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, ruleName string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
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

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, true)
	if err != nil {
		return err
	}

	slog.Debug("Removing rule from rule file...")
	if err := targetsMetadata.RemoveRule(ruleName); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Remove rule '%s' from policy '%s'", ruleName, targetsRoleName)
	return r.updateTargetsMetadata(ctx, state, signer, targetsRoleName, targetsMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddPrincipalToTargets is the interface for a user to add a trusted principal
// to gittuf rule file metadata.
func (r *Repository) AddPrincipalToTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, authorizedPrincipals []tuf.Principal, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
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

	var principalIDs strings.Builder
	for _, principal := range authorizedPrincipals {
		principalIDs.WriteString(fmt.Sprintf("\n%s", principal.ID()))
	}

	slog.Debug("Loading current rule file...")
	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, true)
	if err != nil {
		return err
	}

	for _, principal := range authorizedPrincipals {
		for keyID, principalIDs := range findReusedKeys(targetsMetadata.GetPrincipals(), principal) {
			slog.Warn("Key is already held by another principal, make sure this is intended", "keyID", keyID, "principalID", principal.ID(), "heldBy", principalIDs)
		}

		slog.Debug("Adding principal to rule file", "principalID", strings.TrimSpace(principal.ID()))
		if err := targetsMetadata.AddPrincipal(principal); err != nil {
			return err
		}
	}

	commitMessage := fmt.Sprintf("Add principals to policy '%s'\n%s", targetsRoleName, principalIDs.String())
	return r.updateTargetsMetadata(ctx, state, signer, targetsRoleName, targetsMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// UpdatePrincipalInTargets is the interface for a user to update a principal's
// information in gittuf rule file metadata.
func (r *Repository) UpdatePrincipalInTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, principal tuf.Principal, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	slog.Debug("Loading current rule file...")
	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, true)
	if err != nil {
		return err
	}

	for keyID, principalIDs := range findReusedKeys(targetsMetadata.GetPrincipals(), principal) {
		slog.Warn("Key is already held by another principal, make sure this is intended", "keyID", keyID, "principalID", principal.ID(), "heldBy", principalIDs)
	}

	slog.Debug("Updating principal in rule file", "principalID", strings.TrimSpace(principal.ID()))
	if err := targetsMetadata.UpdatePrincipal(principal); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Update principal '%s' in policy '%s'", principal.ID(), targetsRoleName)
	return r.updateTargetsMetadata(ctx, state, signer, targetsRoleName, targetsMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// RemovePrincipalFromTargets is the interface for a user to remove a principal
// from gittuf rule file metadata.
func (r *Repository) RemovePrincipalFromTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, principalID string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	slog.Debug("Loading current rule file...")
	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, true)
	if err != nil {
		return err
	}

	slog.Debug("Removing principal from rule file", "principalID", strings.TrimSpace(principalID))
	if err := targetsMetadata.RemovePrincipal(principalID); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Remove principal from policy '%s'\n%s", targetsRoleName, principalID)
	return r.updateTargetsMetadata(ctx, state, signer, targetsRoleName, targetsMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

// SignTargets adds a signature to specified Targets role's envelope. Note that
// the metadata itself is not modified, so its version remains the same.
func (r *Repository) SignTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	var env *sslibdsse.Envelope
	if targetsRoleName == policy.TargetsRoleName {
		env = state.Metadata.TargetsEnvelope
	} else {
		env = state.Metadata.DelegationEnvelopes[targetsRoleName]
	}

	slog.Debug("Signing rule file", "keyID", keyID)
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.Metadata.TargetsEnvelope = env
	} else {
		state.Metadata.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Add signature from key '%s' to policy '%s'", keyID, targetsRoleName)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, options.CreateRSLEntry, signCommit)
}

func (r *Repository) IncrementTargetsVersion(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, signCommit bool, opts ...trustpolicyopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &trustpolicyopts.Options{}
	for _, fn := range opts {
		fn(options)
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef, policyopts.BypassRSL())
	if err != nil {
		return err
	}
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	slog.Debug("Loading current rule file...")
	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName, true)
	if err != nil {
		return err
	}

	// Just pass it to updateTargetsMetadata as it will increment the version
	commitMessage := fmt.Sprintf("Increment rule file '%s' version", targetsRoleName)
	return r.updateTargetsMetadata(ctx, state, signer, targetsRoleName, targetsMetadata, commitMessage, options.CreateRSLEntry, signCommit)
}

func (r *Repository) updateTargetsMetadata(ctx context.Context, state *policy.State, signer sslibdsse.SignerVerifier, targetsMetadataName string, targetsMetadata tuf.TargetsMetadata, commitMessage string, createRSLEntry, signCommit bool) error {
	targetsMetadata.IncrementVersion()

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return err
	}

	slog.Debug("Signing updated rule file...")
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if targetsMetadataName == policy.TargetsRoleName {
		state.Metadata.TargetsEnvelope = env
	} else {
		state.Metadata.DelegationEnvelopes[targetsMetadataName] = env
	}

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, createRSLEntry, signCommit)
}

// findReusedKeys returns the keys of the principal that are already used by
// other principals in the rule file, mapped to the IDs of those principals.
func findReusedKeys(existingPrincipals map[string]tuf.Principal, principal tuf.Principal) map[string][]string {
	reusedKeys := map[string][]string{}

	for existingID, existingPrincipal := range existingPrincipals {
		if existingID == principal.ID() {
			continue
		}

		for _, existingKey := range existingPrincipal.Keys() {
			for _, key := range principal.Keys() {
				if key.KeyID == existingKey.KeyID {
					reusedKeys[key.KeyID] = append(reusedKeys[key.KeyID], existingID)
				}
			}
		}
	}

	return reusedKeys
}
