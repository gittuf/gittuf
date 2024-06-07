// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"
	"errors"
	"fmt"
	"path"
	"sort"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/google/go-github/v61/github"
	ita "github.com/in-toto/attestation/go/v1"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	GitHubPullRequestPredicateType         = "https://gittuf.dev/github-pull-request/v0.1"
	GitHubPullRequestApprovalPredicateType = "https://gittuf.dev/github-pull-request-approval/v0.1"
	digestGitCommitKey                     = "gitCommit"
)

var (
	ErrInvalidGitHubPullRequestApprovalAttestation  = errors.New("the GitHub pull request approval attestation does not match expected details")
	ErrGitHubPullRequestApprovalAttestationNotFound = errors.New("requested GitHub pull request approval attestation not found")
)

func NewGitHubPullRequestAttestation(owner, repository string, pullRequestNumber int, commitID string, pullRequest *github.PullRequest) (*ita.Statement, error) {
	pullRequestBytes, err := json.Marshal(pullRequest)
	if err != nil {
		return nil, err
	}

	predicate := map[string]any{}
	if err := json.Unmarshal(pullRequestBytes, &predicate); err != nil {
		return nil, err
	}

	predicateStruct, err := structpb.NewStruct(predicate)
	if err != nil {
		return nil, err
	}

	return &ita.Statement{
		Type: ita.StatementTypeUri,
		Subject: []*ita.ResourceDescriptor{
			{
				Uri:    fmt.Sprintf("https://github.com/%s/%s/pull/%d", owner, repository, pullRequestNumber),
				Digest: map[string]string{digestGitCommitKey: commitID},
			},
		},
		PredicateType: GitHubPullRequestPredicateType,
		Predicate:     predicateStruct,
	}, nil
}

func (a *Attestations) SetGitHubPullRequestAuthorization(repo *gitinterface.Repository, env *sslibdsse.Envelope, targetRefName, commitID string) error {
	envBytes, err := json.Marshal(env)
	if err != nil {
		return err
	}

	blobID, err := repo.WriteBlob(envBytes)
	if err != nil {
		return err
	}

	if a.githubPullRequestAttestations == nil {
		a.githubPullRequestAttestations = map[string]gitinterface.Hash{}
	}

	a.githubPullRequestAttestations[GitHubPullRequestAttestationPath(targetRefName, commitID)] = blobID
	return nil
}

// GitHubPullRequestAttestationPath constructs the expected path on-disk for the
// GitHub pull request attestation.
func GitHubPullRequestAttestationPath(refName, commitID string) string {
	return path.Join(refName, commitID)
}

// GitHubPullRequestApprovalAttestation is similar to a
// `ReferenceAuthorization`, except that it records a pull request's approvers
// inside the predicate (defined here).
type GitHubPullRequestApprovalAttestation struct {
	Approvers []*tuf.Key `json:"approvers"`
	*ReferenceAuthorization
}

// NewGitHubPullRequestApprovalAttestation creates a new GitHub pull request
// approval attestation for the provided information. The attestation is
// embedded in an in-toto "statement" and returned with the appropriate
// "predicate type" set. The `fromTargetID` and `toTargetID` specify the change
// to `targetRef` that is approved on the corresponding GitHub pull request.
func NewGitHubPullRequestApprovalAttestation(targetRef, fromRevisionID, targetTreeID string, approvers []*tuf.Key) (*ita.Statement, error) {
	approversSet := set.NewSet[string]()
	approversFiltered := make([]*tuf.Key, 0, len(approvers))
	for _, approver := range approvers {
		if approversSet.Has(approver.KeyID) {
			continue
		}
		approversSet.Add(approver.KeyID)
		approversFiltered = append(approversFiltered, approver)
	}

	approvers = approversFiltered
	sort.Slice(approvers, func(i, j int) bool {
		return approvers[i].KeyID < approvers[j].KeyID
	})

	predicate := &GitHubPullRequestApprovalAttestation{
		ReferenceAuthorization: &ReferenceAuthorization{
			TargetRef:      targetRef,
			FromRevisionID: fromRevisionID,
			TargetTreeID:   targetTreeID,
		},
		Approvers: approvers,
	}

	predicateStruct, err := predicateToPBStruct(predicate)
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

// SetGitHubPullRequestApprovalAttestation writes the new GitHub pull request
// approval attestation to the object store and tracks it in the current
// attestations state.
func (a *Attestations) SetGitHubPullRequestApprovalAttestation(repo *gitinterface.Repository, env *sslibdsse.Envelope, refName, fromRevisionID, targetTreeID string) error {
	if err := validateGitHubPullRequestApprovalAttestation(env, refName, fromRevisionID, targetTreeID); err != nil {
		return errors.Join(ErrInvalidGitHubPullRequestApprovalAttestation, err)
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		return err
	}

	blobID, err := repo.WriteBlob(envBytes)
	if err != nil {
		return err
	}

	if a.githubPullRequestApprovalAttestations == nil {
		a.githubPullRequestApprovalAttestations = map[string]gitinterface.Hash{}
	}

	a.githubPullRequestApprovalAttestations[GitHubPullRequestApprovalAttestationPath(refName, fromRevisionID, targetTreeID)] = blobID
	return nil
}

// GetGitHubPullRequestApprovalAttestationFor returns the requested GitHub pull
// request approval attestation.
func (a *Attestations) GetGitHubPullRequestApprovalAttestationFor(repo *gitinterface.Repository, refName, fromRevisionID, targetTreeID string) (*sslibdsse.Envelope, error) {
	blobID, has := a.githubPullRequestApprovalAttestations[GitHubPullRequestApprovalAttestationPath(refName, fromRevisionID, targetTreeID)]
	if !has {
		return nil, ErrGitHubPullRequestApprovalAttestationNotFound
	}

	envBytes, err := repo.ReadBlob(blobID)
	if err != nil {
		return nil, err
	}

	env := &sslibdsse.Envelope{}
	if err := json.Unmarshal(envBytes, env); err != nil {
		return nil, err
	}

	return env, nil
}

// GitHubPullRequestApprovalAttestationPath returns the expected path on-disk
// for the GitHub pull request approval attestation. For now, this attestation
// type is stored using the same format as a reference authorization.
func GitHubPullRequestApprovalAttestationPath(refName, fromID, toID string) string {
	return ReferenceAuthorizationPath(refName, fromID, toID)
}

func validateGitHubPullRequestApprovalAttestation(env *sslibdsse.Envelope, targetRef, fromRevisionID, targetTreeID string) error {
	return validateReferenceAuthorization(env, targetRef, fromRevisionID, targetTreeID)
}
