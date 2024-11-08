// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package github

import (
	"errors"

	"github.com/gittuf/gittuf/internal/attestations/authorizations"
)

var (
	ErrInvalidPullRequestApprovalAttestation  = errors.New("the GitHub pull request approval attestation does not match expected details or has no approvers and dismissed approvers")
	ErrPullRequestApprovalAttestationNotFound = errors.New("requested GitHub pull request approval attestation not found")
	ErrGitHubReviewIDNotFound                 = errors.New("requested GitHub review ID does not exist in index")
)

// PullRequestApprovalAttestation records approvals on a GitHub pull request via
// a gittuf GitHub app. It's similar to a Reference Authorization in that it
// records the updated ref, the prior state of the ref, and the target state of
// the ref after the change is made. Unlike a Reference Authorization, it
// records approvers within the predicate. If the app is trusted in the
// repository's root of trust, then the approvers witnessed by the GitHub app
// are trusted during gittuf verification.
type PullRequestApprovalAttestation interface {
	// GetApprovers returns the list of approvers witnessed by the GitHub app.
	GetApprovers() []string

	// GetDismissedApprovers returns the list of approvers who later dismissed
	// their review.
	GetDismissedApprovers() []string

	authorizations.ReferenceAuthorization
}
