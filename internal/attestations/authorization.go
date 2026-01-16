// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/gittuf/gittuf/internal/attestations/authorizations"
	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	authorizationsv02 "github.com/gittuf/gittuf/internal/attestations/authorizations/v02"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	ita "github.com/in-toto/attestation/go/v1"
)

// NewReferenceAuthorizationForCommit creates a new reference authorization for
// the provided information. The authorization is embedded in an in-toto
// "statement" and returned with the appropriate "predicate type" set. The
// `fromID` and `toID` specify the change to `targetRef` that is to be
// authorized by invoking this function. Since this is for a commit, the `toID`
// is expected to be a Git tree ID.
func NewReferenceAuthorizationForCommit(targetRef, fromID, toID string) (*ita.Statement, error) {
	return authorizationsv02.NewReferenceAuthorizationForCommit(targetRef, fromID, toID)
}

// NewReferenceAuthorizationForTag creates a new reference authorization for the
// provided information. The authorization is embedded in an in-toto "statement"
// and returned with the appropriate "predicate type" set. The `fromID` and
// `toID` specify the change to `targetRef` that is to be authorized by invoking
// this function. Since this is for a tag, the `toID` is expected to be a Git
// commit ID.
func NewReferenceAuthorizationForTag(targetRef, fromID, toID string) (*ita.Statement, error) {
	return authorizationsv02.NewReferenceAuthorizationForTag(targetRef, fromID, toID)
}

// SetReferenceAuthorization writes the new reference authorization attestation
// to the object store and tracks it in the current attestations state.
func (a *Attestations) SetReferenceAuthorization(repo *gitinterface.Repository, env *sslibdsse.Envelope, refName, fromID, toID string) error {
	payloadBytes, err := env.DecodeB64Payload()
	if err != nil {
		return fmt.Errorf("unable to inspect reference authorization: %w", err)
	}

	inspectAuthorization := map[string]any{}
	if err := json.Unmarshal(payloadBytes, &inspectAuthorization); err != nil {
		return fmt.Errorf("unable to inspect reference authorization: %w", err)
	}
	switch inspectAuthorization["predicate_type"] {
	case authorizationsv01.PredicateType:
		if err := authorizationsv01.Validate(env, refName, fromID, toID); err != nil {
			return err
		}
	case authorizationsv02.PredicateType:
		if err := authorizationsv02.Validate(env, refName, fromID, toID); err != nil {
			return err
		}
	default:
		return authorizations.ErrUnknownAuthorizationVersion
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		return err
	}

	blobID, err := repo.WriteBlob(envBytes)
	if err != nil {
		return err
	}

	if a.referenceAuthorizations == nil {
		a.referenceAuthorizations = map[string]gitinterface.Hash{}
	}

	a.referenceAuthorizations[ReferenceAuthorizationPath(refName, fromID, toID)] = blobID
	return nil
}

// RemoveReferenceAuthorization removes a set reference authorization
// attestation entirely. The object, however, isn't removed from the object
// store as prior states may still need it.
func (a *Attestations) RemoveReferenceAuthorization(refName, fromID, toID string) error {
	authPath := ReferenceAuthorizationPath(refName, fromID, toID)
	if _, has := a.referenceAuthorizations[authPath]; !has {
		return authorizations.ErrAuthorizationNotFound
	}

	delete(a.referenceAuthorizations, authPath)
	return nil
}

// GetReferenceAuthorizationFor returns the requested reference authorization
// attestation (with its signatures).
func (a *Attestations) GetReferenceAuthorizationFor(repo *gitinterface.Repository, refName, fromID, toID string) (*sslibdsse.Envelope, error) {
	blobID, has := a.referenceAuthorizations[ReferenceAuthorizationPath(refName, fromID, toID)]
	if !has {
		return nil, authorizations.ErrAuthorizationNotFound
	}

	envBytes, err := repo.ReadBlob(blobID)
	if err != nil {
		return nil, err
	}

	env := &sslibdsse.Envelope{}
	if err := json.Unmarshal(envBytes, env); err != nil {
		return nil, err
	}

	payloadBytes, err := env.DecodeB64Payload()
	if err != nil {
		return nil, fmt.Errorf("unable to inspect reference authorization: %w", err)
	}

	inspectAuthorization := map[string]any{}
	if err := json.Unmarshal(payloadBytes, &inspectAuthorization); err != nil {
		return nil, fmt.Errorf("unable to inspect reference authorization: %w", err)
	}
	switch inspectAuthorization["predicate_type"] {
	case authorizationsv01.PredicateType:
		if err := authorizationsv01.Validate(env, refName, fromID, toID); err != nil {
			return nil, err
		}
	case authorizationsv02.PredicateType:
		if err := authorizationsv02.Validate(env, refName, fromID, toID); err != nil {
			return nil, err
		}
	default:
		return nil, authorizations.ErrUnknownAuthorizationVersion
	}

	return env, nil
}

// ReferenceAuthorizationPath constructs the expected path on-disk for the
// reference authorization attestation.
func ReferenceAuthorizationPath(refName, fromID, toID string) string {
	return path.Join(refName, fmt.Sprintf("%s-%s", fromID, toID))
}
