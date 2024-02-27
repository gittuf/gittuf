// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

var ErrInvalidPolicyName = errors.New("invalid rule or policy file name, cannot be 'root'")

// InitializeTargets is the interface for the user to create the specified
// policy file.
func (r *Repository) InitializeTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, signCommit bool) error {
	if targetsRoleName == policy.RootRoleName {
		return ErrInvalidPolicyName
	}

	state, err := policy.LoadCurrentState(ctx, r.r)
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

	targetsMetadata := policy.InitializeTargetsMetadata()

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return nil
	}

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return nil
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

	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// AddDelegation is the interface for the user to add a new rule to gittuf
// policy.
func (r *Repository) AddDelegation(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, ruleName string, authorizedKeys []*tuf.Key, rulePatterns []string, signCommit bool) error {
	if ruleName == policy.RootRoleName {
		return ErrInvalidPolicyName
	}

	state, err := policy.LoadCurrentState(ctx, r.r)
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

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName)
	if err != nil {
		return err
	}

	targetsMetadata, err = policy.AddOrUpdateDelegation(targetsMetadata, ruleName, authorizedKeys, rulePatterns)
	if err != nil {
		return err
	}

	targetsMetadata.SetVersion(targetsMetadata.Version + 1)

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return nil
	}

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return nil
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Add rule '%s' to policy '%s'", ruleName, targetsRoleName)

	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// RemoveDelegation is the interface for a user to remove a rule from gittuf
// policy.
func (r *Repository) RemoveDelegation(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, ruleName string, signCommit bool) error {
	state, err := policy.LoadCurrentState(ctx, r.r)
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

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName)
	if err != nil {
		return err
	}

	targetsMetadata, err = policy.RemoveDelegation(targetsMetadata, ruleName)
	if err != nil {
		return err
	}

	targetsMetadata.SetVersion(targetsMetadata.Version + 1)

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return nil
	}

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return nil
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Remove rule '%s' from policy '%s'", ruleName, targetsRoleName)

	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// AddKeyToTargets is the interface for a user to add a trusted key to the
// gittuf policy.
func (r *Repository) AddKeyToTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, authorizedKeys []*tuf.Key, signCommit bool) error {
	state, err := policy.LoadCurrentState(ctx, r.r)
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

	keyIDs := ""
	for _, key := range authorizedKeys {
		keyIDs += fmt.Sprintf("\n%s:%s", key.KeyType, key.KeyID)
	}

	targetsMetadata, err := state.GetTargetsMetadata(targetsRoleName)
	if err != nil {
		return err
	}

	targetsMetadata, err = policy.AddKeyToTargets(targetsMetadata, authorizedKeys)
	if err != nil {
		return err
	}

	targetsMetadata.SetVersion(targetsMetadata.Version + 1)

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

	commitMessage := fmt.Sprintf("Add keys to policy '%s'\n%s", targetsRoleName, keyIDs)

	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// SignTargets adds a signature to specified Targets role's envelope. Note that
// the metadata itself is not modified, so its version remains the same.
func (r *Repository) SignTargets(ctx context.Context, signer sslibdsse.SignerVerifier, targetsRoleName string, signCommit bool) error {
	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}
	if !state.HasTargetsRole(targetsRoleName) {
		return policy.ErrMetadataNotFound
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	var env *sslibdsse.Envelope
	if targetsRoleName == policy.TargetsRoleName {
		env = state.TargetsEnvelope
	} else {
		env = state.DelegationEnvelopes[targetsRoleName]
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

	commitMessage := fmt.Sprintf("Add signature from key '%s' to policy '%s'", keyID, targetsRoleName)

	return state.Commit(ctx, r.r, commitMessage, signCommit)
}
