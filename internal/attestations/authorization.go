// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/gittuf/gittuf/internal/attestations/authorizations"
	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	"github.com/gittuf/gittuf/internal/gitinterface"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	ita "github.com/in-toto/attestation/go/v1"
)

// NewReferenceAuthorization creates a new reference authorization for the
// provided information. The authorization is embedded in an in-toto "statement"
// and returned with the appropriate "predicate type" set. The `fromID` and
// `toID` specify the change to `targetRef` that is to be authorized by invoking
// this function.
func NewReferenceAuthorization(targetRef, fromID, toID string) (*ita.Statement, error) {
	return authorizationsv01.NewReferenceAuthorization(targetRef, fromID, toID)
}

// SetReferenceAuthorization writes the new reference authorization attestation
// to the object store and tracks it in the current attestations state.
func (a *Attestations) SetReferenceAuthorization(repo *gitinterface.Repository, env *sslibdsse.Envelope, refName, fromID, toID string) error {
	// TODO: we'll probably support validating multiple versions here for cross
	// compatibility
	if err := authorizationsv01.Validate(env, refName, fromID, toID); err != nil {
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

	// TODO: this will probably be updated to support multiple versions as
	// they're introduced
	if err := authorizationsv01.Validate(env, refName, fromID, toID); err != nil {
		return nil, err
	}

	return env, nil
}

// ReferenceAuthorizationPath constructs the expected path on-disk for the
// reference authorization attestation.
func ReferenceAuthorizationPath(refName, fromID, toID string) string {
	return path.Join(refName, fmt.Sprintf("%s-%s", fromID, toID))
}
