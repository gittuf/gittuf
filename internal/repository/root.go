// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	d "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

// InitializeRoot is the interface for the user to create the repository's root
// of trust.
func (r *Repository) InitializeRoot(ctx context.Context, rootKeyBytes []byte, signCommit bool) error {
	if err := r.InitializeNamespaces(); err != nil {
		return err
	}

	publicKey, err := tuf.LoadKeyFromBytes(rootKeyBytes)
	if err != nil {
		return err
	}
	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes)
	if err != nil {
		return err
	}

	rootMetadata := policy.InitializeRootMetadata(publicKey)

	env, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		return nil
	}

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return nil
	}

	state := &policy.State{
		RootPublicKeys: []*tuf.Key{publicKey},
		RootEnvelope:   env,
	}

	commitMessage := "Initialize root of trust"

	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// AddTopLevelTargetsKey is the interface for the user to add an authorized key
// for the top level Targets role / policy file.
func (r *Repository) AddTopLevelTargetsKey(ctx context.Context, rootKeyBytes, targetsKeyBytes []byte, signCommit bool) error {
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes)
	if err != nil {
		return err
	}
	rootKeyID, err := sv.KeyID()
	if err != nil {
		return err
	}

	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}

	rootMetadata, err := state.GetRootMetadata()
	if err != nil {
		return err
	}

	if !isKeyAuthorized(rootMetadata.Roles[policy.RootRoleName].KeyIDs, rootKeyID) {
		return ErrUnauthorizedKey
	}

	targetsKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
	if err != nil {
		return err
	}

	rootMetadata = policy.AddTargetsKey(rootMetadata, targetsKey)

	rootMetadata.SetVersion(rootMetadata.Version + 1)
	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []d.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	env, err = dsse.SignEnvelope(ctx, env, sv)
	if err != nil {
		return err
	}

	state.RootEnvelope = env

	commitMessage := fmt.Sprintf("Add policy key '%s' to root", targetsKey.KeyID)

	return state.Commit(ctx, r.r, commitMessage, signCommit)
}

// RemoveTopLevelTargetsKey is the interface for the user to de-authorize a key
// trusted to sign the top level Targets role / policy file.
func (r *Repository) RemoveTopLevelTargetsKey(ctx context.Context, rootKeyBytes []byte, targetsKeyID string, signCommit bool) error {
	sv, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes)
	if err != nil {
		return err
	}
	rootKeyID, err := sv.KeyID()
	if err != nil {
		return err
	}

	state, err := policy.LoadCurrentState(ctx, r.r)
	if err != nil {
		return err
	}

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

	rootMetadata.SetVersion(rootMetadata.Version + 1)
	rootMetadataBytes, err := json.Marshal(rootMetadata)
	if err != nil {
		return err
	}

	env := state.RootEnvelope
	env.Signatures = []d.Signature{}
	env.Payload = base64.StdEncoding.EncodeToString(rootMetadataBytes)

	env, err = dsse.SignEnvelope(ctx, env, sv)
	if err != nil {
		return err
	}

	state.RootEnvelope = env

	commitMessage := fmt.Sprintf("Remove policy key '%s' from root", targetsKeyID)

	return state.Commit(ctx, r.r, commitMessage, signCommit)
}
