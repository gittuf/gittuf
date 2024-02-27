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
	slog.Debug("Initialized gittuf namespaces")

	rawKey := signer.Public()
	publicKey, err := sslibsv.NewKey(rawKey)
	if err != nil {
		return err
	}
	slog.Debug("Loaded root signer")

	rootMetadata := policy.InitializeRootMetadata(publicKey)
	slog.Debug("Initialized root metadata")

	env, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		return nil
	}
	slog.Debug("Created DSSE envelope with root metadata")

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return nil
	}
	slog.Debug("Signed DSSE envelope via signer verifier")

	state := &policy.State{
		RootPublicKeys: []*tuf.Key{publicKey},
		RootEnvelope:   env,
	}

	commitMessage := "Initialize root of trust"

	err = state.Commit(ctx, r.r, commitMessage, signCommit)
	if err != nil {
		return err
	}
	slog.Debug("Committed policy state")

	return nil
}

// AddRootKey is the interface for the user to add an authorized key
// for the Root role.
func (r *Repository) AddRootKey(ctx context.Context, signer sslibdsse.SignerVerifier, newRootKey *tuf.Key, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}
	slog.Debug("Loaded current policy state")

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKeyID) {
		return ErrUnauthorizedKey
	}

	rootMetadata = policy.AddRootKey(rootMetadata, newRootKey)

	rootMetadata.SetVersion(rootMetadata.Version + 1)
	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []sslibdsse.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}
	slog.Debug("Signed DSSE envelope via signer verifier")

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

	err = state.Commit(ctx, r.r, commitMessage, signCommit)
	if err != nil {
		return err
	}
	slog.Debug("Committed policy state")

	return nil
}

// RemoveRootKey is the interface for the user to de-authorize a key
// trusted to sign the Root role.
func (r *Repository) RemoveRootKey(ctx context.Context, signer sslibdsse.SignerVerifier, keyID string, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}
	slog.Debug("Loaded current policy state")

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKeyID) {
		return ErrUnauthorizedKey
	}

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

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}
	slog.Debug("Signed DSSE envelope via signer verifier")

	newRootPublicKeys := []*tuf.Key{}
	for _, key := range state.RootPublicKeys {
		if key.KeyID != keyID {
			newRootPublicKeys = append(newRootPublicKeys, key)
		}
	}

	state.RootEnvelope = env
	state.RootPublicKeys = newRootPublicKeys

	commitMessage := fmt.Sprintf("Remove root key '%s' from root", keyID)

	err = state.Commit(ctx, r.r, commitMessage, signCommit)
	if err != nil {
		return err
	}
	slog.Debug("Committed policy state")

	return nil
}

// AddTopLevelTargetsKey is the interface for the user to add an authorized key
// for the top level Targets role / policy file.
func (r *Repository) AddTopLevelTargetsKey(ctx context.Context, signer sslibdsse.SignerVerifier, targetsKey *tuf.Key, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}
	slog.Debug("Loaded current policy state")

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKeyID) {
		return ErrUnauthorizedKey
	}

	rootMetadata = policy.AddTargetsKey(rootMetadata, targetsKey)
	slog.Debug("Added policy key to the gittuf root", "ID", targetsKey.KeyID)

	rootMetadata.SetVersion(rootMetadata.Version + 1)
	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []sslibdsse.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}
	slog.Debug("Signed DSSE envelope via signer verifier")

	state.RootEnvelope = env

	commitMessage := fmt.Sprintf("Add policy key '%s' to root", targetsKey.KeyID)

	err = state.Commit(ctx, r.r, commitMessage, signCommit)
	if err != nil {
		return err
	}
	slog.Debug("Committed policy state")

	return nil
}

// RemoveTopLevelTargetsKey is the interface for the user to de-authorize a key
// trusted to sign the top level Targets role / policy file.
func (r *Repository) RemoveTopLevelTargetsKey(ctx context.Context, signer sslibdsse.SignerVerifier, targetsKeyID string, signCommit bool) error {
	rootKeyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}
	slog.Debug("Loaded policy state")

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKeyID) {
		return ErrUnauthorizedKey
	}

	rootMetadata, err = policy.DeleteTargetsKey(rootMetadata, targetsKeyID)
	if err != nil {
		return err
	}
	slog.Debug("Removed policy key from gittuf root", "ID", targetsKeyID)

	rootMetadata.SetVersion(rootMetadata.Version + 1)

	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []sslibdsse.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}
	slog.Debug("Signed DSSE envelope via signer verifier")

	state.RootEnvelope = env

	commitMessage := fmt.Sprintf("Remove policy key '%s' from root", targetsKeyID)

	err = state.Commit(ctx, r.r, commitMessage, signCommit)
	if err != nil {
		return err
	}

	slog.Debug("Committed policy state")

	return nil
}
