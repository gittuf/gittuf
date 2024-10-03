// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"errors"
	"sort"
	"testing"

	authorizationsv01 "github.com/gittuf/gittuf/internal/attestations/authorizations/v01"
	"github.com/gittuf/gittuf/internal/attestations/common"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	ita "github.com/in-toto/attestation/go/v1"
)

const (
	GitHubPullRequestApprovalPredicateType = "https://gittuf.dev/github-pull-request-approval/v0.1"

	digestGitTreeKey = "gitTree"
)

var ErrInvalidGitHubPullRequestApprovalAttestation = errors.New("the GitHub pull request approval attestation does not match expected details or has no approvers and dismissed approvers")

// GitHubPullRequestApprovalAttestation is similar to a
// `ReferenceAuthorization`, except that it records a pull request's approvers
// inside the predicate (defined here).
type GitHubPullRequestApprovalAttestation struct {
	// Approvers contains the list of currently applicable approvers.
	Approvers []*tuf.Key `json:"approvers"`

	// DismissedApprovers contains the list of approvers who then dismissed
	// their approval.
	DismissedApprovers []*tuf.Key `json:"dismissedApprovers"`

	*authorizationsv01.ReferenceAuthorization
}

func (g *GitHubPullRequestApprovalAttestation) GetApprovers() []*tuf.Key {
	return g.Approvers
}

func (g *GitHubPullRequestApprovalAttestation) GetDismissedApprovers() []*tuf.Key {
	return g.DismissedApprovers
}

// NewGitHubPullRequestApprovalAttestation creates a new GitHub pull request
// approval attestation for the provided information. The attestation is
// embedded in an in-toto "statement" and returned with the appropriate
// "predicate type" set. The `fromTargetID` and `toTargetID` specify the change
// to `targetRef` that is approved on the corresponding GitHub pull request.
func NewGitHubPullRequestApprovalAttestation(targetRef, fromRevisionID, targetTreeID string, approvers []*tuf.Key, dismissedApprovers []*tuf.Key) (*ita.Statement, error) {
	if len(approvers) == 0 && len(dismissedApprovers) == 0 {
		return nil, ErrInvalidGitHubPullRequestApprovalAttestation
	}

	approvers = getFilteredSetOfApprovers(approvers)
	dismissedApprovers = getFilteredSetOfApprovers(dismissedApprovers)

	predicate := &GitHubPullRequestApprovalAttestation{
		ReferenceAuthorization: &authorizationsv01.ReferenceAuthorization{
			TargetRef:      targetRef,
			FromRevisionID: fromRevisionID,
			TargetTreeID:   targetTreeID,
		},
		Approvers:          approvers,
		DismissedApprovers: dismissedApprovers,
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
		PredicateType: GitHubPullRequestApprovalPredicateType,
		Predicate:     predicateStruct,
	}, nil
}

func ValidatePullRequestApproval(env *sslibdsse.Envelope, targetRef, fromRevisionID, targetTreeID string) error {
	return authorizationsv01.Validate(env, targetRef, fromRevisionID, targetTreeID)
}

func getFilteredSetOfApprovers(approvers []*tuf.Key) []*tuf.Key {
	if approvers == nil {
		return nil
	}
	approversSet := set.NewSet[string]()
	approversFiltered := make([]*tuf.Key, 0, len(approvers))
	for _, approver := range approvers {
		if approversSet.Has(approver.KeyID) {
			continue
		}
		approversSet.Add(approver.KeyID)
		approversFiltered = append(approversFiltered, approver)
	}

	sort.Slice(approversFiltered, func(i, j int) bool {
		return approversFiltered[i].KeyID < approversFiltered[j].KeyID
	})

	return approversFiltered
}

func CreateTestPullRequestApprovalEnvelope(t *testing.T, refName, fromID, toID string, approvers []*tuf.Key) *sslibdsse.Envelope {
	t.Helper()

	authorization, err := NewGitHubPullRequestApprovalAttestation(refName, fromID, toID, approvers, nil)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(authorization)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
