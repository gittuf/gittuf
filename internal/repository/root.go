// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// InitializeRoot is the interface for the user to create the repository's root
// of trust.
func (r *Repository) InitializeRoot(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool) error {
	if err := r.InitializeNamespaces(); err != nil {
		return err
	}

	rawKey := signer.Public()
	publicKey, err := sslibsv.NewKey(rawKey)
	if err != nil {
		return err
	}

	slog.Debug("Creating initial root metadata...")
	rootMetadata := policy.InitializeRootMetadata(publicKey)

	env, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		return nil
	}

	slog.Debug(fmt.Sprintf("Signing initial root metadata using '%s'...", publicKey.KeyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return nil
	}

	state := &policy.State{
		RootPublicKeys: []*tuf.Key{publicKey},
		RootEnvelope:   env,
	}

	commitMessage := "Initialize root of trust"

	slog.Debug("Committing policy...")
	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// AddRootKey is the interface for the user to add an authorized key
// for the Root role.
func (r *Repository) AddRootKey(ctx context.Context, signer sslibdsse.SignerVerifier, newRootKey *tuf.Key, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}

	slog.Debug("Loading current root metadata...")
	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKeyID) {
		return ErrUnauthorizedKey
	}

	slog.Debug("Adding root key...")
	rootMetadata = policy.AddRootKey(rootMetadata, newRootKey)

	rootMetadata.SetVersion(rootMetadata.Version + 1)
	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []sslibdsse.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	slog.Debug(fmt.Sprintf("Signing updated root metadata using '%s'...", rootKeyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	state.RootEnvelope = env

	found := false
	for _, key := range state.RootPublicKeys {
		if key.KeyID == newRootKey.KeyID {
			found = true
			break
		}
	}
	if !found {
		state.RootPublicKeys = append(state.RootPublicKeys, newRootKey)
	}

	commitMessage := fmt.Sprintf("Add root key '%s' to root", newRootKey.KeyID)

	slog.Debug("Committing policy...")
	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// RemoveRootKey is the interface for the user to de-authorize a key
// trusted to sign the Root role.
func (r *Repository) RemoveRootKey(ctx context.Context, signer sslibdsse.SignerVerifier, keyID string, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}

	slog.Debug("Loading current root metadata...")
	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKeyID) {
		return ErrUnauthorizedKey
	}

	slog.Debug("Removing root key...")
	rootMetadata, err = policy.DeleteRootKey(rootMetadata, keyID)
	if err != nil {
		return err
	}

	rootMetadata.SetVersion(rootMetadata.Version + 1)
	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []sslibdsse.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	slog.Debug(fmt.Sprintf("Signing updated root metadata using '%s'...", rootKeyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	newRootPublicKeys := []*tuf.Key{}
	for _, key := range state.RootPublicKeys {
		if key.KeyID != keyID {
			newRootPublicKeys = append(newRootPublicKeys, key)
		}
	}

	state.RootEnvelope = env
	state.RootPublicKeys = newRootPublicKeys

	commitMessage := fmt.Sprintf("Remove root key '%s' from root", keyID)

	slog.Debug("Committing policy...")
	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// AddTopLevelTargetsKey is the interface for the user to add an authorized key
// for the top level Targets role / policy file.
func (r *Repository) AddTopLevelTargetsKey(ctx context.Context, signer sslibdsse.SignerVerifier, targetsKey *tuf.Key, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}

	slog.Debug("Loading current root metadata...")
	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKeyID) {
		return ErrUnauthorizedKey
	}

	slog.Debug("Adding policy key...")
	rootMetadata, err = policy.AddTargetsKey(rootMetadata, targetsKey)
	if err != nil {
		return fmt.Errorf("failed to add policy key: %w", err)
	}

	rootMetadata.SetVersion(rootMetadata.Version + 1)
	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []sslibdsse.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	slog.Debug(fmt.Sprintf("Signing updated root metadata using '%s'...", rootKeyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	state.RootEnvelope = env

	commitMessage := fmt.Sprintf("Add policy key '%s' to root", targetsKey.KeyID)

	slog.Debug("Committing policy...")
	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// RemoveTopLevelTargetsKey is the interface for the user to de-authorize a key
// trusted to sign the top level Targets role / policy file.
func (r *Repository) RemoveTopLevelTargetsKey(ctx context.Context, signer sslibdsse.SignerVerifier, targetsKeyID string, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}

	slog.Debug("Loading current root metadata...")
	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKeyID) {
		return ErrUnauthorizedKey
	}

	slog.Debug("Removing policy key...")
	rootMetadata, err = policy.DeleteTargetsKey(rootMetadata, targetsKeyID)
	if err != nil {
		return err
	}

	rootMetadata.SetVersion(rootMetadata.Version + 1)

	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []sslibdsse.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	slog.Debug(fmt.Sprintf("Signing updated root metadata using '%s'...", rootKeyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	state.RootEnvelope = env

	commitMessage := fmt.Sprintf("Remove policy key '%s' from root", targetsKeyID)

	slog.Debug("Committing policy...")
	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// UpdateTopLevelTargetsThreshold sets the threshold of valid signatures
// required for the top level Targets role.
func (r *Repository) UpdateTopLevelTargetsThreshold(ctx context.Context, signer sslibdsse.SignerVerifier, threshold int, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}

	slog.Debug("Loading current root metadata...")
	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKeyID) {
		return ErrUnauthorizedKey
	}

	slog.Debug("Updating policy threshold...")
	rootMetadata, err = policy.UpdateTargetsThreshold(rootMetadata, threshold)
	if err != nil {
		return err
	}

	rootMetadata.SetVersion(rootMetadata.Version + 1)
	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []sslibdsse.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	slog.Debug(fmt.Sprintf("Signing updated root metadata using '%s'...", rootKeyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	state.RootEnvelope = env

	commitMessage := fmt.Sprintf("Update policy threshold to %d", threshold)

	slog.Debug("Committing policy...")
	return state.Commit(ctx, r.r, commitMessage, signCommit)
}
