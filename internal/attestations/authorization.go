// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"testing"

	// this is imported without versioning because we can just bump the version
	// in the import for the creation flow when there's a new default version
	authorizations "github.com/gittuf/gittuf/internal/attestations/authorizations/v02" //nolint:stylecheck

	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	authorizationsv02 "github.com/gittuf/gittuf/internal/attestations/authorizations/v02" //nolint:stylecheck
	"github.com/gittuf/gittuf/internal/gitinterface"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	ita "github.com/in-toto/attestation/go/v1"
)

var (
	ErrAuthorizationNotFound       = errors.New("requested authorization not found")
	ErrUnknownAuthorizationVersion = errors.New("unknown authorizations version (do you need to update your gittuf client?)")
)

// NewReferenceAuthorizationForCommit creates a new reference authorization for
// the provided information. The authorization is embedded in an in-toto
// "statement" and returned with the appropriate "predicate type" set. The
// `fromID` and `targetID` specify the change to `targetRef` that is to be
// authorized by invoking this function. The targetID is expected to be the Git
// tree ID of the resultant commit.
func NewReferenceAuthorizationForCommit(targetRef, fromID, targetID string) (*ita.Statement, error) {
	return authorizations.NewReferenceAuthorizationForCommit(targetRef, fromID, targetID)
}

// NewReferenceAuthorizationForTag creates a new reference authorization for the
// provided information. The authorization is embedded in an in-toto "statement"
// and returned with the appropriate "predicate type" set. The `fromID` and
// `targetID` specify the change to `targetRef` that is to be authorized by
// invoking this function. The targetID is expected to be the ID of the commit
// the tag will point to.
func NewReferenceAuthorizationForTag(targetRef, fromID, targetID string) (*ita.Statement, error) {
	return authorizations.NewReferenceAuthorizationForTag(targetRef, fromID, targetID)
}

// SetReferenceAuthorization writes the new reference authorization attestation
// to the object store and tracks it in the current attestations state.
func (a *Attestations) SetReferenceAuthorization(repo *gitinterface.Repository, env *sslibdsse.Envelope, refName, fromID, targetID string) error {
	// We assume that since we're setting a new authorization, it's the latest
	// version
	if err := authorizations.Validate(env, refName, fromID, targetID); err != nil {
		return err
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

	a.referenceAuthorizations[ReferenceAuthorizationPath(refName, fromID, targetID)] = blobID
	return nil
}

// SetReferenceAuthorizationWithoutValidating writes the new reference
// authorization attestation to the object store and tracks it in the current
// attestations state. It skips validation of the attestation, and is therefore
// only meant for use during testing to check we support older versions
// correctly.
func (a *Attestations) SetReferenceAuthorizationWithoutValidating(t *testing.T, repo *gitinterface.Repository, env *sslibdsse.Envelope, refName, fromID, targetID string) {
	t.Helper()

	envBytes, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}

	blobID, err := repo.WriteBlob(envBytes)
	if err != nil {
		t.Fatal(err)
	}

	if a.referenceAuthorizations == nil {
		a.referenceAuthorizations = map[string]gitinterface.Hash{}
	}

	a.referenceAuthorizations[ReferenceAuthorizationPath(refName, fromID, targetID)] = blobID
}

// RemoveReferenceAuthorization removes a set reference authorization
// attestation entirely. The object, however, isn't removed from the object
// store as prior states may still need it.
func (a *Attestations) RemoveReferenceAuthorization(refName, fromRevisionID, targetTreeID string) error {
	authPath := ReferenceAuthorizationPath(refName, fromRevisionID, targetTreeID)
	if _, has := a.referenceAuthorizations[authPath]; !has {
		return ErrAuthorizationNotFound
	}

	delete(a.referenceAuthorizations, authPath)
	return nil
}

// GetReferenceAuthorizationFor returns the requested reference authorization
// attestation (with its signatures).
func (a *Attestations) GetReferenceAuthorizationFor(repo *gitinterface.Repository, refName, fromID, targetID string) (*sslibdsse.Envelope, error) {
	blobID, has := a.referenceAuthorizations[ReferenceAuthorizationPath(refName, fromID, targetID)]
	if !has {
		return nil, ErrAuthorizationNotFound
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
		return nil, err
	}

	type tmpStmt struct {
		PredicateType string `json:"predicate_type"`
	}
	stmt := new(tmpStmt)
	if err := json.Unmarshal(payloadBytes, stmt); err != nil {
		return nil, err
	}

	// Inspect predicate type to use appropriate validator
	switch stmt.PredicateType {
	case authorizationsv01.ReferenceAuthorizationPredicateType:
		if err := authorizationsv01.Validate(env, refName, fromID, targetID); err != nil {
			return nil, err
		}
	case authorizationsv02.ReferenceAuthorizationPredicateType:
		if err := authorizationsv02.Validate(env, refName, fromID, targetID); err != nil {
			return nil, err
		}
	default:
		return nil, ErrUnknownAuthorizationVersion
	}

	return env, nil
}

// ReferenceAuthorizationPath constructs the expected path on-disk for the
// reference authorization attestation.
func ReferenceAuthorizationPath(refName, fromID, toID string) string {
	return path.Join(refName, fmt.Sprintf("%s-%s", fromID, toID))
}
