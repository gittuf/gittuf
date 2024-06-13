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
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
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

	var (
		publicKey *sslibsv.SSLibKey
		err       error
	)
	if signerSSH, isSSHSigner := signer.(*ssh.Signer); isSSHSigner {
		publicKey = signerSSH.MetadataKey()
	} else {
		rawKey := signer.Public()
		publicKey, err = sslibsv.NewKey(rawKey)
		if err != nil {
			return err
		}
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
	return state.Commit(r.r, commitMessage, signCommit)
}

// AddRootKey is the interface for the user to add an authorized key
// for the Root role.
func (r *Repository) AddRootKey(ctx context.Context, signer sslibdsse.SignerVerifier, newRootKey *tuf.Key, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Adding root key...")
	rootMetadata = policy.AddRootKey(rootMetadata, newRootKey)

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
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// RemoveRootKey is the interface for the user to de-authorize a key
// trusted to sign the Root role.
func (r *Repository) RemoveRootKey(ctx context.Context, signer sslibdsse.SignerVerifier, keyID string, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Removing root key...")
	rootMetadata, err = policy.DeleteRootKey(rootMetadata, keyID)
	if err != nil {
		return err
	}

	newRootPublicKeys := []*tuf.Key{}
	for _, key := range state.RootPublicKeys {
		if key.KeyID != keyID {
			newRootPublicKeys = append(newRootPublicKeys, key)
		}
	}
	state.RootPublicKeys = newRootPublicKeys

	commitMessage := fmt.Sprintf("Remove root key '%s' from root", keyID)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// AddTopLevelTargetsKey is the interface for the user to add an authorized key
// for the top level Targets role / policy file.
func (r *Repository) AddTopLevelTargetsKey(ctx context.Context, signer sslibdsse.SignerVerifier, targetsKey *tuf.Key, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Adding policy key...")
	rootMetadata, err = policy.AddTargetsKey(rootMetadata, targetsKey)
	if err != nil {
		return fmt.Errorf("failed to add policy key: %w", err)
	}

	commitMessage := fmt.Sprintf("Add policy key '%s' to root", targetsKey.KeyID)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// RemoveTopLevelTargetsKey is the interface for the user to de-authorize a key
// trusted to sign the top level Targets role / policy file.
func (r *Repository) RemoveTopLevelTargetsKey(ctx context.Context, signer sslibdsse.SignerVerifier, targetsKeyID string, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Removing policy key...")
	rootMetadata, err = policy.DeleteTargetsKey(rootMetadata, targetsKeyID)
	if err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Remove policy key '%s' from root", targetsKeyID)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// UpdateRootThreshold sets the threshold of valid signatures required for the
// Root role.
func (r *Repository) UpdateRootThreshold(ctx context.Context, signer sslibdsse.SignerVerifier, threshold int, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Updating root threshold...")
	rootMetadata, err = policy.UpdateRootThreshold(rootMetadata, threshold)
	if err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Update root threshold to %d", threshold)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// UpdateTopLevelTargetsThreshold sets the threshold of valid signatures
// required for the top level Targets role.
func (r *Repository) UpdateTopLevelTargetsThreshold(ctx context.Context, signer sslibdsse.SignerVerifier, threshold int, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	rootMetadata, err := r.loadRootMetadata(state, rootKeyID)
	if err != nil {
		return err
	}

	slog.Debug("Updating policy threshold...")
	rootMetadata, err = policy.UpdateTargetsThreshold(rootMetadata, threshold)
	if err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Update policy threshold to %d", threshold)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// SignRoot adds a signature to the Root envelope. Note that the metadata itself
// is not modified, so its version remains the same.
func (r *Repository) SignRoot(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool) error {
	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
	if err != nil {
		return err
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	env := state.RootEnvelope

	slog.Debug(fmt.Sprintf("Signing root metadata using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	state.RootEnvelope = env

	commitMessage := fmt.Sprintf("Add signature from key '%s' to root metadata", keyID)

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}

func (r *Repository) loadRootMetadata(state *policy.State, keyID string) (*tuf.RootMetadata, error) {
	slog.Debug("Loading current root metadata...")
	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return nil, err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, keyID) {
		return nil, ErrUnauthorizedKey
	}

	return rootMetadata, nil
}

func (r *Repository) updateRootMetadata(ctx context.Context, state *policy.State, signer sslibdsse.SignerVerifier, rootMetadata *tuf.RootMetadata, commitMessage string, signCommit bool) error {
	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []sslibdsse.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	slog.Debug("Signing updated root metadata...")
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	state.RootEnvelope = env

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}
