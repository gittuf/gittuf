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
	toTargetIDKey              = "toTargetID"
	fromTargetIDKey            = "fromTargetID"
	targetRefKey               = "targetRef"
)

var (
	ErrInvalidAuthorization  = errors.New("authorization attestation does not match expected details")
	ErrAuthorizationNotFound = errors.New("requested authorization not found")
)

// Authorization is a lightweight record of a detached authorization in a gittuf
// repository. It is meant to be used as a "predicate" in an in-toto
// attestation.
type Authorization struct {
	TargetRef    string `json:"targetRef"`
	FromTargetID string `json:"fromTargetID"`
	ToTargetID   string `json:"toTargetID"`
}

// NewAuthorizationAttestation creates a new authorization for the provided
// information. The authorization is embedded in an in-toto "statement" and
// returned with the appropriate "predicate type" set. The `fromTargetID` and
// `toTargetID` specify the change to `targetRef` that is to be authorized by
// invoking this function.
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

// SetAuthorizationAttestation writes the new authorization attestation to the
// object store and tracks it in the current attestations state.
func (a *Attestations) SetAuthorizationAttestation(repo *git.Repository, env *sslibdsse.Envelope, refName, fromID, toID string) error {
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

	a.authorizations[AuthorizationPath(refName, fromID, toID)] = blobID
	return nil
}

// RemoveAuthorizationAttestation removes a set authorization attestation
// entirely. The object, however, isn't removed from the object store as prior
// states may still need it.
func (a *Attestations) RemoveAuthorizationAttestation(refName, fromID, toID string) error {
	authPath := AuthorizationPath(refName, fromID, toID)
	if _, has := a.authorizations[authPath]; !has {
		return ErrAuthorizationNotFound
	}

	delete(a.authorizations, authPath)
	return nil
}

// GetAuthorizationAttestationFor returns the requested authorization
// attestation (with its signatures).
func (a *Attestations) GetAuthorizationAttestationFor(repo *git.Repository, refName, fromID, toID string) (*sslibdsse.Envelope, error) {
	blobID, has := a.authorizations[AuthorizationPath(refName, fromID, toID)]
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

	if err := validateAuthorization(env, refName, fromID, toID); err != nil {
		return nil, err
	}

	return env, nil
}

// AuthorizationPath constructs the expected path on-disk for the authorization
// attestation.
func AuthorizationPath(refName, fromID, toID string) string {
	return path.Join(refName, fmt.Sprintf("%s-%s", fromID, toID))
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

	if attestation.Subject[0].Digest[digestGitCommitKey] != toID {
		return ErrInvalidAuthorization
	}

	predicate := attestation.Predicate.AsMap()

	if predicate[toTargetIDKey] != toID {
		return ErrInvalidAuthorization
	}

	if predicate[fromTargetIDKey] != fromID {
		return ErrInvalidAuthorization
	}

	if predicate[targetRefKey] != refName {
		return ErrInvalidAuthorization
	}

	return nil
}
