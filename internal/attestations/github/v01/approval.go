// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	"github.com/gittuf/gittuf/internal/attestations/common"
	"github.com/gittuf/gittuf/internal/attestations/github"
	"github.com/gittuf/gittuf/internal/common/set"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	ita "github.com/in-toto/attestation/go/v1"
)

const (
	PullRequestApprovalPredicateType = "https://gittuf.dev/github-pull-request-approval/v0.1"

	digestGitTreeKey = "gitTree"
)

type PullRequestApprovalAttestation struct {
	// Approvers contains the list of currently applicable approvers.
	Approvers *set.Set[string] `json:"approvers"`

	// DismissedApprovers contains the list of approvers who then dismissed
	// their approval.
	DismissedApprovers *set.Set[string] `json:"dismissedApprovers"`

	*authorizationsv01.ReferenceAuthorization
}

func (pra *PullRequestApprovalAttestation) GetApprovers() []string {
	return pra.Approvers.Contents()
}

func (pra *PullRequestApprovalAttestation) GetDismissedApprovers() []string {
	return pra.DismissedApprovers.Contents()
}

// NewPullRequestApprovalAttestation creates a new GitHub pull request approval
// attestation for the provided information. The attestation is embedded in an
// in-toto "statement" and returned with the appropriate "predicate type" set.
// The `fromTargetID` and `toTargetID` specify the change to `targetRef` that is
// approved on the corresponding GitHub pull request.
func NewPullRequestApprovalAttestation(targetRef, fromRevisionID, targetTreeID string, approvers, dismissedApprovers []string) (*ita.Statement, error) {
	if len(approvers) == 0 && len(dismissedApprovers) == 0 {
		return nil, github.ErrInvalidPullRequestApprovalAttestation
	}

	predicate := &PullRequestApprovalAttestation{
		ReferenceAuthorization: &authorizationsv01.ReferenceAuthorization{
			TargetRef:      targetRef,
			FromRevisionID: fromRevisionID,
			TargetTreeID:   targetTreeID,
		},
		Approvers:          set.NewSetFromItems(approvers...),
		DismissedApprovers: set.NewSetFromItems(dismissedApprovers...),
	}

	predicateStruct, err := common.PredicateToPBStruct(predicate)
	if err != nil {
		return nil, err
	}

	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				Digest: map[string]string{digestGitTreeKey: targetTreeID},
			},
		},
		PredicateType: PullRequestApprovalPredicateType,
		Predicate:     predicateStruct,
	}, nil
}

func ValidatePullRequestApproval(env *sslibdsse.Envelope, targetRef, fromRevisionID, targetTreeID string) error {
	return authorizationsv01.Validate(env, targetRef, fromRevisionID, targetTreeID)
}
