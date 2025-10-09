// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package authorizations

import "errors"

var (
	ErrInvalidHatAttestation = errors.New("authorization attestation does not match expected details")
)

// HatAttestation represents an attestation that declares which team a person
// is making a commit on behalf of.
type HatAttestation interface {
	// GetRef returns the reference for the change approved by the attestation.
	GetRef() string

	// GetTargetID returns the Git ID of the commit in question.
	GetTargetID() string

	// GetTeamID returns the ID of the team that the change is being made on
	// behalf of.
	GetTeamID() string
}
