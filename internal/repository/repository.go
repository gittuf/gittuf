package repository

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/policy"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/adityasaky/gittuf/internal/signerverifier"
	"github.com/adityasaky/gittuf/internal/signerverifier/dsse"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
	d "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

var ErrUnauthorizedRootKey = errors.New("unauthorized root key presented when updating root of trust")

type Repository struct {
	r *git.Repository
}

func LoadRepository() (*Repository, error) {
	repo, err := common.GetRepositoryHandler()
	if err != nil {
		return nil, err
	}

	return &Repository{
		r: repo,
	}, nil
}

func (r *Repository) InitializeNamespaces() error {
	if err := rsl.InitializeNamespace(r.r); err != nil {
		return err
	}

	return policy.InitializeNamespace(r.r)
}

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
	return state.Commit(ctx, r.r, signCommit)
}

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

	if !isRootKeyAuthorized(rootMetadata.Roles[policy.RootRoleName], rootKeyID) {
		return ErrUnauthorizedRootKey
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
	return state.Commit(ctx, r.r, signCommit)
}

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

	if !isRootKeyAuthorized(rootMetadata.Roles[policy.RootRoleName], rootKeyID) {
		return ErrUnauthorizedRootKey
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
	return state.Commit(ctx, r.r, signCommit)
}

func isRootKeyAuthorized(rootRole tuf.Role, keyID string) bool {
	for _, k := range rootRole.KeyIDs {
		if k == keyID {
			return true
		}
	}
	return false
}
