// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package authorizations

import "errors"

var (
	ErrInvalidAuthorization        = errors.New("authorization attestation does not match expected details")
	ErrAuthorizationNotFound       = errors.New("requested authorization not found")
	ErrUnknownAuthorizationVersion = errors.New("unknown reference authorization version")
)

// ReferenceAuthorization represents an attestation that approves a change to a
// reference.
type ReferenceAuthorization interface {
	// GetRef returns the reference for the change approved by the attestation.
	GetRef() string

	// GetFromID returns the Git ID of the reference prior to the change.
	GetFromID() string

	// GetTargetID returns the Git ID of the reference after the change is
	// applied. Note that this is typically something that can be pre-computed,
	// such as the Git tree ID for a merge that has not happened yet.
	GetTargetID() string

	// GetTeamID returns the ID of the team that the change is being made on
	// behalf of.
	GetTeamID() string
}
