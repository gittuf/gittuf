// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"sort"

	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	"github.com/gittuf/gittuf/internal/attestations/common"
	"github.com/gittuf/gittuf/internal/attestations/github"
	"github.com/gittuf/gittuf/internal/common/set"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	ita "github.com/in-toto/attestation/go/v1"
)

const (
	PullRequestApprovalPredicateType = "https://gittuf.dev/github-pull-request-approval/v0.1"

	digestGitTreeKey = "gitTree"
)

type PullRequestApprovalAttestation struct {
	// Approvers contains the list of currently applicable approvers.
	Approvers []*tufv01.Key `json:"approvers"`

	// DismissedApprovers contains the list of approvers who then dismissed
	// their approval.
	DismissedApprovers []*tufv01.Key `json:"dismissedApprovers"`

	*authorizationsv01.ReferenceAuthorization
}

func (pra *PullRequestApprovalAttestation) GetApprovers() []*tufv01.Key {
	return pra.Approvers
}

func (pra *PullRequestApprovalAttestation) GetDismissedApprovers() []*tufv01.Key {
	return pra.DismissedApprovers
}

// NewPullRequestApprovalAttestation creates a new GitHub pull request approval
// attestation for the provided information. The attestation is embedded in an
// in-toto "statement" and returned with the appropriate "predicate type" set.
// The `fromTargetID` and `toTargetID` specify the change to `targetRef` that is
// approved on the corresponding GitHub pull request.
func NewPullRequestApprovalAttestation(targetRef, fromRevisionID, targetTreeID string, approvers, dismissedApprovers []tuf.Principal) (*ita.Statement, error) {
	if len(approvers) == 0 && len(dismissedApprovers) == 0 {
		return nil, github.ErrInvalidPullRequestApprovalAttestation
	}

	// TODO: all of this is temporary until we can just record a principal's app
	// specific ID. We can't just switch to that schema here because the policy
	// package relies on it at the moment, so that'll happen together.
	approvers = getFilteredSetOfApprovers(approvers)
	dismissedApprovers = getFilteredSetOfApprovers(dismissedApprovers)

	approversTyped := make([]*tufv01.Key, 0, len(approvers))
	for _, approver := range approvers {
		approverTyped, isKnownType := approver.(*tufv01.Key)
		if !isKnownType {
			return nil, tuf.ErrInvalidPrincipalType
		}
		approversTyped = append(approversTyped, approverTyped)
	}

	dismissedApproversTyped := make([]*tufv01.Key, 0, len(dismissedApprovers))
	for _, dismissedApprover := range dismissedApprovers {
		dismissedApproverTyped, isKnownType := dismissedApprover.(*tufv01.Key)
		if !isKnownType {
			return nil, tuf.ErrInvalidPrincipalType
		}
		dismissedApproversTyped = append(dismissedApproversTyped, dismissedApproverTyped)
	}

	predicate := &PullRequestApprovalAttestation{
		ReferenceAuthorization: &authorizationsv01.ReferenceAuthorization{
			TargetRef:      targetRef,
			FromRevisionID: fromRevisionID,
			TargetTreeID:   targetTreeID,
		},
		Approvers:          approversTyped,
		DismissedApprovers: dismissedApproversTyped,
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

func getFilteredSetOfApprovers(approvers []tuf.Principal) []tuf.Principal {
	// TODO: this will be removed when we just use a set of principal IDs in the
	// predicate directly

	if approvers == nil {
		return nil
	}
	approversSet := set.NewSet[string]()
	approversFiltered := make([]tuf.Principal, 0, len(approvers))
	for _, approver := range approvers {
		if approversSet.Has(approver.ID()) {
			continue
		}
		approversSet.Add(approver.ID())
		approversFiltered = append(approversFiltered, approver)
	}

	sort.Slice(approversFiltered, func(i, j int) bool {
		return approversFiltered[i].ID() < approversFiltered[j].ID()
	})

	return approversFiltered
}
