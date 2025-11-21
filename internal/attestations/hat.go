// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"
	"fmt"
	"path"

	"github.com/gittuf/gittuf/internal/attestations/hat"
	hatv01 "github.com/gittuf/gittuf/internal/attestations/hat/v01"
	"github.com/gittuf/gittuf/internal/gitinterface"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	ita "github.com/in-toto/attestation/go/v1"
)

// NewHatAttestationForCommit creates a new hat attestation for the provided
// information. The hat attestation is embedded in an in-toto "statement" and
// returned with the appropriate "predicate type" set. The `targetID` and
// `teamID` specify the change itself, and the team that the user is attesting on
// behalf of. Since this is for a commit, the targetID is expected to be a Git
// Tree ID.
func NewHatAttestationForCommit(targetRef, targetID, teamID string) (*ita.Statement, error) {
	return hatv01.NewHatAttestationForCommit(targetRef, targetID, teamID)
}

// NewHatAttestationForTag creates a new hat attestation for the provided
// information. The hat attestation is embedded in an in-toto "statement" and
// returned with the appropriate "predicate type" set. The `targetID` and
// `teamID` specify the change itself, and the team that the user is attesting on
// behalf of. Since this is for a tag, the targetID is expected to be a Git
// commit ID.
func NewHatAttestationForTag(targetRef, targetID, teamID string) (*ita.Statement, error) {
	return hatv01.NewHatAttestationForTag(targetRef, targetID, teamID)
}

// SetHatAttestation writes the new hat attestation to the object store and
// tracks it in the current attestations state.
func (a *Attestations) SetHatAttestation(repo *gitinterface.Repository, env *sslibdsse.Envelope, refName, targetID, teamID string) error {
	payloadBytes, err := env.DecodeB64Payload()
	if err != nil {
		return fmt.Errorf("unable to inspect hat attestation: %w", err)
	}

	inspectHatAttestation := map[string]any{}
	if err := json.Unmarshal(payloadBytes, &inspectHatAttestation); err != nil {
		return fmt.Errorf("unable to inspect hat attestation: %w", err)
	}

	if err := hatv01.Validate(env, refName, targetID, teamID); err != nil {
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

	if a.HatAttestations == nil {
		a.HatAttestations = map[string]gitinterface.Hash{}
	}

	// TODO: clearly define mapping for hat attestations;; refname + targetid + hat?
	a.HatAttestations[ReferenceAuthorizationPath(refName, targetID, teamID)] = blobID
	return nil
}

// RemoveHatAttestation removes a set hat attestation entirely. The object,
// however, isn't removed from the object store as prior states may still need
// it.
func (a *Attestations) RemoveHatAttestation(refName, targetID, teamID string) error {
	hatPath := HatAttestationPath(refName, targetID, teamID)
	if _, has := a.HatAttestations[hatPath]; !has {
		return hat.ErrHatAttestationNotFound
	}

	delete(a.HatAttestations, hatPath)
	return nil
}

// GetHatAttestationFor returns the requested hat attestation (with its
// signatures).
func (a *Attestations) GetHatAttestationFor(repo *gitinterface.Repository, refName, targetID, teamID string) (*sslibdsse.Envelope, error) {
	blobID, has := a.HatAttestations[HatAttestationPath(refName, targetID, teamID)]
	if !has {
		return nil, hat.ErrHatAttestationNotFound
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
		return nil, fmt.Errorf("unable to inspect hat attestation: %w", err)
	}
	inspectHatAttestation := map[string]any{}
	if err := json.Unmarshal(payloadBytes, &inspectHatAttestation); err != nil {
		return nil, fmt.Errorf("unable to inspect hat attestation: %w", err)
	}

	if err := hatv01.Validate(env, refName, targetID, teamID); err != nil {
		return nil, err
	}

	return env, nil
}

// HatAttestationPath constructs the expected path on-disk for the hat
// attestation.
func HatAttestationPath(refName, targetID, teamID string) string {
	return path.Join(refName, fmt.Sprintf("%s-%s", targetID, teamID))
}
