// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	ita "github.com/in-toto/attestation/go/v1"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	AuthorizationPredicateType = "https://gittuf.dev/authorization/v0.1"
	digestGitCommitKey         = "gitCommit"
)

var (
	ErrInvalidAuthorization  = errors.New("authorization attestation does not match expected details")
	ErrAuthorizationNotFound = errors.New("requested authorization not found")
)

type Authorization struct {
	TargetRef    string `json:"targetRef"`
	FromTargetID string `json:"fromTargetID"`
	ToTargetID   string `json:"toTargetID"`
}

func NewAuthorizationAttestation(targetRef, fromTargetID, toTargetID string) (*ita.Statement, error) {
	predicate := &Authorization{
		TargetRef:    targetRef,
		FromTargetID: fromTargetID,
		ToTargetID:   toTargetID,
	}

	predicateBytes, err := json.Marshal(predicate)
	if err != nil {
		return nil, err
	}

	predicateInterface := &map[string]any{}
	if err := json.Unmarshal(predicateBytes, predicateInterface); err != nil {
		return nil, err
	}

	predicateStruct, err := structpb.NewStruct(*predicateInterface)
	if err != nil {
		return nil, err
	}

	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				Digest: map[string]string{digestGitCommitKey: toTargetID},
			},
		},
		PredicateType: AuthorizationPredicateType,
		Predicate:     predicateStruct,
	}, nil
}

func (a *Attestations) AddAuthorizationAttestation(repo *git.Repository, env *sslibdsse.Envelope, refName, fromID, toID string) error {
	if err := validateAuthorization(env, refName, fromID, toID); err != nil {
		return err
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		return err
	}

	blobID, err := gitinterface.WriteBlob(repo, envBytes)
	if err != nil {
		return err
	}

	if a.authorizations == nil {
		a.authorizations = map[string]plumbing.Hash{}
	}

	a.authorizations[authorizationPath(refName, fromID, toID)] = blobID
	return nil
}

func (a *Attestations) RemoveAuthorization(repo *git.Repository, refName, fromID, toID, keyID string) error {
	env, err := a.GetAuthorizationAttestationFor(repo, refName, fromID, toID)
	if err != nil {
		return err
	}

	if err := validateAuthorization(env, refName, fromID, toID); err != nil {
		return err
	}

	signatures := []sslibdsse.Signature{}
	for _, sig := range env.Signatures {
		if sig.KeyID != keyID {
			signatures = append(signatures, sig)
		}
	}

	if len(signatures) == len(env.Signatures) {
		// can't revoke a signature that doesn't exist
		return ErrAuthorizationNotFound
	}

	if len(signatures) == 0 {
		delete(a.authorizations, authorizationPath(refName, fromID, toID))
		return nil
	}

	// Write new env with reduced number of signatures
	env.Signatures = signatures
	envBytes, err := json.Marshal(env)
	if err != nil {
		return err
	}

	blobID, err := gitinterface.WriteBlob(repo, envBytes)
	if err != nil {
		return err
	}

	a.authorizations[authorizationPath(refName, fromID, toID)] = blobID
	return nil
}

func (a *Attestations) GetAuthorizationAttestationFor(repo *git.Repository, refName, fromID, toID string) (*sslibdsse.Envelope, error) {
	blobID, has := a.authorizations[authorizationPath(refName, fromID, toID)]
	if !has {
		return nil, ErrAuthorizationNotFound
	}

	envBytes, err := gitinterface.ReadBlob(repo, blobID)
	if err != nil {
		return nil, err
	}

	env := &sslibdsse.Envelope{}
	if err := json.Unmarshal(envBytes, env); err != nil {
		return nil, err
	}

	return env, nil
}

func validateAuthorization(env *sslibdsse.Envelope, refName, fromID, toID string) error {
	payload, err := env.DecodeB64Payload()
	if err != nil {
		return err
	}

	attestation := &ita.Statement{}
	if err := json.Unmarshal(payload, attestation); err != nil {
		return err
	}

	if attestation.Subject[0].Digest["gitCommit"] != toID {
		return ErrInvalidAuthorization
	}

	predicate := attestation.Predicate.AsMap()

	if predicate["toTargetID"] != toID {
		return ErrInvalidAuthorization
	}

	if predicate["fromTargetID"] != fromID {
		return ErrInvalidAuthorization
	}

	if predicate["targetRef"] != refName {
		return ErrInvalidAuthorization
	}

	return nil
}

func authorizationPath(refName, fromID, toID string) string {
	return path.Join(refName, fmt.Sprintf("%s-%s", fromID, toID))
}
