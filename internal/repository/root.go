// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

// InitializeRoot is the interface for the user to create the repository's root
// of trust.
func (r *Repository) InitializeRoot(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool) error {
	var (
		publicKeyRaw *signerverifier.SSLibKey
		err          error
	)
	switch signer := signer.(type) {
	case *ssh.Signer:
		publicKeyRaw = signer.MetadataKey()
	case *sigstore.Signer:
		publicKeyRaw, err = signer.MetadataKey()
		if err != nil {
			return err
		}
	default:
		return common.ErrUnknownKeyType
	}
	publicKey := tufv01.NewKeyFromSSLibKey(publicKeyRaw)

	slog.Debug("Creating initial root metadata...")
	rootMetadata, err := policy.InitializeRootMetadata(publicKey)
	if err != nil {
		return err
	}

	env, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing initial root metadata using '%s'...", publicKey.KeyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	state := &policy.State{
		RootPublicKeys: []tuf.Principal{publicKey},
		RootEnvelope:   env,
	}

	commitMessage := "Initialize root of trust"

	slog.Debug("Committing policy...")
	return state.Commit(r.r, commitMessage, signCommit)
}

// AddRootKey is the interface for the user to add an authorized key
// for the Root role.
func (r *Repository) AddRootKey(ctx context.Context, signer sslibdsse.SignerVerifier, newRootKey tuf.Principal, signCommit bool) error {
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
	if err := rootMetadata.AddRootPrincipal(newRootKey); err != nil {
		return err
	}

	found := false
	for _, key := range state.RootPublicKeys {
		if key.ID() == newRootKey.ID() {
			found = true
			break
		}
	}
	if !found {
		state.RootPublicKeys = append(state.RootPublicKeys, newRootKey)
	}

	commitMessage := fmt.Sprintf("Add root key '%s' to root", newRootKey.ID())
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
	if err := rootMetadata.DeleteRootPrincipal(keyID); err != nil {
		return err
	}

	newRootPublicKeys := []tuf.Principal{}
	for _, key := range state.RootPublicKeys {
		if key.ID() != keyID {
			newRootPublicKeys = append(newRootPublicKeys, key)
		}
	}
	state.RootPublicKeys = newRootPublicKeys

	commitMessage := fmt.Sprintf("Remove root key '%s' from root", keyID)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// AddTopLevelTargetsKey is the interface for the user to add an authorized key
// for the top level Targets role / policy file.
func (r *Repository) AddTopLevelTargetsKey(ctx context.Context, signer sslibdsse.SignerVerifier, targetsKey tuf.Principal, signCommit bool) error {
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
	if err := rootMetadata.AddPrimaryRuleFilePrincipal(targetsKey); err != nil {
		return fmt.Errorf("failed to add policy key: %w", err)
	}

	commitMessage := fmt.Sprintf("Add policy key '%s' to root", targetsKey.ID())
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
	if err := rootMetadata.DeletePrimaryRuleFilePrincipal(targetsKeyID); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Remove policy key '%s' from root", targetsKeyID)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// AddGitHubAppKey is the interface for the user to add the authorized key for
// the special GitHub app role. This key is used to verify GitHub pull request
// approval attestation signatures.
func (r *Repository) AddGitHubAppKey(ctx context.Context, signer sslibdsse.SignerVerifier, appKey tuf.Principal, signCommit bool) error {
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

	slog.Debug("Adding GitHub app key...")
	if err := rootMetadata.AddGitHubAppPrincipal(appKey); err != nil {
		return fmt.Errorf("failed to add GitHub app key: %w", err)
	}

	commitMessage := fmt.Sprintf("Add GitHub app key '%s' to root", appKey.ID())
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// RemoveGitHubAppKey is the interface for the user to de-authorize the key for
// the special GitHub app role.
func (r *Repository) RemoveGitHubAppKey(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool) error {
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

	slog.Debug("Removing GitHub app key...")
	rootMetadata.DeleteGitHubAppPrincipal()

	commitMessage := "Remove GitHub app key from root"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// TrustGitHubApp updates the root metadata to mark GitHub app pull request
// approvals as trusted.
func (r *Repository) TrustGitHubApp(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool) error {
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

	if rootMetadata.IsGitHubAppApprovalTrusted() {
		slog.Debug("GitHub app approvals are already trusted, exiting...")
		return nil
	}

	slog.Debug("Marking GitHub app approvals as trusted in root...")
	rootMetadata.EnableGitHubAppApprovals()

	commitMessage := "Mark GitHub app approvals as trusted"
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// UntrustGitHubApp updates the root metadata to mark GitHub app pull request
// approvals as untrusted.
func (r *Repository) UntrustGitHubApp(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool) error {
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

	if !rootMetadata.IsGitHubAppApprovalTrusted() {
		slog.Debug("GitHub app approvals are already untrusted, exiting...")
		return nil
	}

	slog.Debug("Marking GitHub app approvals as untrusted in root...")
	rootMetadata.DisableGitHubAppApprovals()

	commitMessage := "Mark GitHub app approvals as untrusted"
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
	if err := rootMetadata.UpdateRootThreshold(threshold); err != nil {
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
	if err := rootMetadata.UpdatePrimaryRuleFileThreshold(threshold); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Update policy threshold to %d", threshold)
	return r.updateRootMetadata(ctx, state, signer, rootMetadata, commitMessage, signCommit)
}

// SignRoot adds a signature to the Root envelope. Note that the metadata itself
// is not modified, so its version remains the same.
func (r *Repository) SignRoot(ctx context.Context, signer sslibdsse.SignerVerifier, signCommit bool) error {
	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug("Loading current policy...")
	state, err := policy.LoadCurrentState(ctx, r.r, policy.PolicyStagingRef)
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

func (r *Repository) loadRootMetadata(state *policy.State, keyID string) (tuf.RootMetadata, error) {
	slog.Debug("Loading current root metadata...")
	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return nil, err
	}

	authorizedPrincipals, err := rootMetadata.GetRootPrincipals()
	if err != nil {
		return nil, err
	}

	if !isKeyAuthorized(authorizedPrincipals, keyID) {
		return nil, ErrUnauthorizedKey
	}

	return rootMetadata, nil
}

func (r *Repository) updateRootMetadata(ctx context.Context, state *policy.State, signer sslibdsse.SignerVerifier, rootMetadata tuf.RootMetadata, commitMessage string, signCommit bool) error {
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
