// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"
	"sort"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/google/go-github/v61/github"
	ita "github.com/in-toto/attestation/go/v1"
	"google.golang.org/protobuf/types/known/structpb"
)

const (
	GitHubPullRequestPredicateType         = "https://gittuf.dev/github-pull-request/v0.1"
	GitHubPullRequestApprovalPredicateType = "https://gittuf.dev/github-pull-request-approval/v0.1"
	digestGitCommitKey                     = "gitCommit"
)

var (
	ErrInvalidGitHubPullRequestApprovalAttestation  = errors.New("the GitHub pull request approval attestation does not match expected details or has no approvers and dismissed approvers")
	ErrGitHubPullRequestApprovalAttestationNotFound = errors.New("requested GitHub pull request approval attestation not found")
	ErrGitHubReviewIDNotFound                       = errors.New("requested GitHub review ID does not exist in index")
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
	// Approvers contains the list of currently applicable approvers.
	Approvers []*tuf.Key `json:"approvers"`

	// DismissedApprovers contains the list of approvers who then dismissed
	// their approval.
	DismissedApprovers []*tuf.Key `json:"dismissedApprovers"`

	*ReferenceAuthorization
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
		ReferenceAuthorization: &ReferenceAuthorization{
			TargetRef:      targetRef,
			FromRevisionID: fromRevisionID,
			TargetTreeID:   targetTreeID,
		},
		Approvers:          approvers,
		DismissedApprovers: dismissedApprovers,
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
// attestations state. The refName, fromRevisionID, targetTreeID parameters are
// used to construct an indexPath. The hostURL and reviewID are together mapped
// to the indexPath so that if the review is dismissed later, the corresponding
// attestation can be updated.
func (a *Attestations) SetGitHubPullRequestApprovalAttestation(repo *gitinterface.Repository, env *sslibdsse.Envelope, hostURL string, reviewID int64, appName, refName, fromRevisionID, targetTreeID string) error {
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

	if a.codeReviewApprovalAttestations == nil {
		a.codeReviewApprovalAttestations = map[string]gitinterface.Hash{}
	}

	if a.codeReviewApprovalIndex == nil {
		a.codeReviewApprovalIndex = map[string]string{}
	}

	indexPath := GitHubPullRequestApprovalAttestationPath(refName, fromRevisionID, targetTreeID)
	// We URL encode the appName to make it appropriate for an on-disk path
	blobPath := path.Join(indexPath, base64.URLEncoding.EncodeToString([]byte(appName)))

	// Note the distinction between indexPath and blobPath
	// We don't have this for reference authorizations
	// indexPath is of the form "<ref>/<from commit>-<target tree>/github"
	// blobPath is a specific entry in the indexPath tree, for the app recording
	// the attestation

	a.codeReviewApprovalAttestations[blobPath] = blobID

	githubReviewID, err := GitHubReviewID(hostURL, reviewID)
	if err != nil {
		return err
	}
	if existingIndexPath, has := a.codeReviewApprovalIndex[githubReviewID]; has {
		if existingIndexPath != indexPath {
			return ErrInvalidGitHubPullRequestApprovalAttestation
		}
	} else {
		a.codeReviewApprovalIndex[githubReviewID] = indexPath // only use indexPath as the same review ID can be observed by more than one app
	}

	return nil
}

// GetGitHubPullRequestApprovalAttestationFor returns the requested GitHub pull
// request approval attestation. Here, all the pieces of information to load the
// attestation are known: the change the approval is for as well as the app that
// observed the approval.
func (a *Attestations) GetGitHubPullRequestApprovalAttestationFor(repo *gitinterface.Repository, appName, refName, fromRevisionID, targetTreeID string) (*sslibdsse.Envelope, error) {
	indexPath := GitHubPullRequestApprovalAttestationPath(refName, fromRevisionID, targetTreeID)
	return a.GetGitHubPullRequestApprovalAttestationForIndexPath(repo, appName, indexPath)
}

// GetGitHubPullRequestApprovalAttestationForReviewID returns the requested
// GitHub pull request approval attestation for the specified GitHub instance,
// review ID, and app. This is used when the indexPath is unknown, such as when
// dismissing a prior approval. The host information and reviewID are used to
// identify the indexPath for the requested review.
func (a *Attestations) GetGitHubPullRequestApprovalAttestationForReviewID(repo *gitinterface.Repository, hostURL string, reviewID int64, appName string) (*sslibdsse.Envelope, error) {
	indexPath, has, err := a.GetGitHubPullRequestApprovalIndexPathForReviewID(hostURL, reviewID)
	if err != nil {
		return nil, err
	}
	if has {
		return a.GetGitHubPullRequestApprovalAttestationForIndexPath(repo, appName, indexPath)
	}

	return nil, ErrGitHubReviewIDNotFound
}

// GetGitHubPullRequestApprovalAttestationForIndexPath returns the requested
// GitHub pull request approval attestation for the indexPath and appName.
func (a *Attestations) GetGitHubPullRequestApprovalAttestationForIndexPath(repo *gitinterface.Repository, appName, indexPath string) (*sslibdsse.Envelope, error) {
	// We URL encode the appName to match the on-disk path
	blobPath := path.Join(indexPath, base64.URLEncoding.EncodeToString([]byte(appName)))
	blobID, has := a.codeReviewApprovalAttestations[blobPath]
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

// GetGitHubPullRequestApprovalIndexPathForReviewID uses the host and review ID
// to find the previously recorded index path. Also see:
// SetGitHubPullRequestApprovalAttestation.
func (a *Attestations) GetGitHubPullRequestApprovalIndexPathForReviewID(hostURL string, reviewID int64) (string, bool, error) {
	githubReviewID, err := GitHubReviewID(hostURL, reviewID)
	if err != nil {
		return "", false, err
	}
	indexPath, has := a.codeReviewApprovalIndex[githubReviewID]
	return indexPath, has, nil
}

// GitHubPullRequestApprovalAttestationPath returns the expected path on-disk
// for the GitHub pull request approval attestation. This attestation type is
// stored using the same format as a reference authorization with the addition
// of `github` at the end of the path. This must be used as the tree to store
// specific attestation blobs in.
func GitHubPullRequestApprovalAttestationPath(refName, fromID, toID string) string {
	return path.Join(ReferenceAuthorizationPath(refName, fromID, toID), githubPullRequestApprovalSystemName)
}

// GitHubReviewID converts a GitHub specific review ID (recorded as an int64
// number by GitHub) into a code review system agnostic identifier used by
// gittuf.
func GitHubReviewID(hostURL string, reviewID int64) (string, error) {
	u, err := url.Parse(hostURL)
	if err != nil {
		return "", err
	}

	return fmt.Sprintf("%s::%d", u.Host, reviewID), nil
}

func validateGitHubPullRequestApprovalAttestation(env *sslibdsse.Envelope, targetRef, fromRevisionID, targetTreeID string) error {
	return validateReferenceAuthorization(env, targetRef, fromRevisionID, targetTreeID)
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
