// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"fmt"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// InitializeTargets is the interface for the user to create the specified
// policy file.
func (r *Repository) InitializeTargets(ctx context.Context, targetsKeyBytes []byte, targetsRoleName string, signCommit bool) error {
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes)
	if err != nil {
		return err
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
	// See: https://github.com/theupdateframework/specification/issues/19,
	// https://github.com/theupdateframework/specification/issues/214, and
	// https://github.com/theupdateframework/python-tuf/issues/660.

	targetsMetadata := policy.InitializeTargetsMetadata()

	env, err := dsse.CreateEnvelope(targetsMetadata)
	if err != nil {
		return nil
	}

	env, err = dsse.SignEnvelope(ctx, env, sv)
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
func (r *Repository) AddDelegation(ctx context.Context, signingKeyBytes []byte, targetsRoleName string, ruleName string, authorizedKeysBytes [][]byte, rulePatterns []string, signCommit bool) error {
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(signingKeyBytes)
	if err != nil {
		return err
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
	// See: https://github.com/theupdateframework/specification/issues/19,
	// https://github.com/theupdateframework/specification/issues/214, and
	// https://github.com/theupdateframework/python-tuf/issues/660.

	authorizedKeys := []*tuf.Key{}
	for _, kb := range authorizedKeysBytes {
		key, err := tuf.LoadKeyFromBytes(kb)
		if err != nil {
			return err
		}

		authorizedKeys = append(authorizedKeys, key)
	}

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

	env, err = dsse.SignEnvelope(ctx, env, sv)
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
func (r *Repository) RemoveDelegation(ctx context.Context, signingKeyBytes []byte, targetsRoleName string, ruleName string, signCommit bool) error {
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(signingKeyBytes)
	if err != nil {
		return err
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
	// See: https://github.com/theupdateframework/specification/issues/19,
	// https://github.com/theupdateframework/specification/issues/214, and
	// https://github.com/theupdateframework/python-tuf/issues/660.

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

	env, err = dsse.SignEnvelope(ctx, env, sv)
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
func (r *Repository) AddKeyToTargets(ctx context.Context, signingKeyBytes []byte, targetsRoleName string, authorizedKeysBytes [][]byte, signCommit bool) error {
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(signingKeyBytes)
	if err != nil {
		return err
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
	// See: https://github.com/theupdateframework/specification/issues/19,
	// https://github.com/theupdateframework/specification/issues/214, and
	// https://github.com/theupdateframework/python-tuf/issues/660.

	authorizedKeys := []*tuf.Key{}
	keyIDs := ""
	for _, kb := range authorizedKeysBytes {
		key, err := tuf.LoadKeyFromBytes(kb)
		if err != nil {
			return err
		}

		authorizedKeys = append(authorizedKeys, key)
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
		return nil
	}

	env, err = dsse.SignEnvelope(ctx, env, sv)
	if err != nil {
		return nil
	}

	if targetsRoleName == policy.TargetsRoleName {
		state.TargetsEnvelope = env
	} else {
		state.DelegationEnvelopes[targetsRoleName] = env
	}

	commitMessage := fmt.Sprintf("Add keys to policy '%s'\n%s", targetsRoleName, keyIDs)

	return state.Commit(ctx, r.r, commitMessage, signCommit)
}
