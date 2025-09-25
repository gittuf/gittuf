// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"path"

	"github.com/gittuf/gittuf/internal/attestations/github"
	githubv01 "github.com/gittuf/gittuf/internal/attestations/github/v01"
	"github.com/gittuf/gittuf/internal/gitinterface"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	gogithub "github.com/google/go-github/v61/github"
	ita "github.com/in-toto/attestation/go/v1"
)

func NewGitHubPullRequestAttestation(owner, repository string, pullRequestNumber int, commitID string, pullRequest *gogithub.PullRequest) (*ita.Statement, error) {
	return githubv01.NewPullRequestAttestation(owner, repository, pullRequestNumber, commitID, pullRequest)
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

// NewGitHubPullRequestApprovalAttestation creates a new GitHub pull request
// approval attestation for the provided information. The attestation is
// embedded in an in-toto "statement" and returned with the appropriate
// "predicate type" set. The `fromTargetID` and `toTargetID` specify the change
// to `targetRef` that is approved on the corresponding GitHub pull request.
func NewGitHubPullRequestApprovalAttestation(targetRef, fromRevisionID, targetTreeID string, approvers, dismissedApprovers []string) (*ita.Statement, error) {
	return githubv01.NewPullRequestApprovalAttestation(targetRef, fromRevisionID, targetTreeID, approvers, dismissedApprovers)
}

// SetGitHubPullRequestApprovalAttestation writes the new GitHub pull request
// approval attestation to the object store and tracks it in the current
// attestations state. The refName, fromRevisionID, targetTreeID parameters are
// used to construct an indexPath. The hostURL and reviewID are together mapped
// to the indexPath so that if the review is dismissed later, the corresponding
// attestation can be updated.
func (a *Attestations) SetGitHubPullRequestApprovalAttestation(repo *gitinterface.Repository, env *sslibdsse.Envelope, hostURL string, reviewID int64, appName, refName, fromRevisionID, targetTreeID string) (gitinterface.Hash, error) {
	// TODO: this will be updated to support validating different versions
	if err := githubv01.ValidatePullRequestApproval(env, refName, fromRevisionID, targetTreeID); err != nil {
		return nil, errors.Join(github.ErrInvalidPullRequestApprovalAttestation, err)
	}

	envBytes, err := json.Marshal(env)
	if err != nil {
		return nil, err
	}

	blobID, err := repo.WriteBlob(envBytes)
	if err != nil {
		return nil, err
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
		return nil, err
	}
	if existingIndexPath, has := a.codeReviewApprovalIndex[githubReviewID]; has {
		if existingIndexPath != indexPath {
			return nil, github.ErrInvalidPullRequestApprovalAttestation
		}
	} else {
		a.codeReviewApprovalIndex[githubReviewID] = indexPath // only use indexPath as the same review ID can be observed by more than one app
	}

	return blobID, nil
}

// GetGitHubPullRequestApprovalAttestationFor returns the requested GitHub pull
// request approval attestation. Here, all the pieces of information to load the
// attestation are known: the change the approval is for as well as the app that
// observed the approval.
func (a *Attestations) GetGitHubPullRequestApprovalAttestationFor(repo *gitinterface.Repository, appName, refName, fromRevisionID, targetTreeID string) (*sslibdsse.Envelope, gitinterface.Hash, error) {
	indexPath := GitHubPullRequestApprovalAttestationPath(refName, fromRevisionID, targetTreeID)
	return a.GetGitHubPullRequestApprovalAttestationForIndexPath(repo, appName, indexPath)
}

// GetGitHubPullRequestApprovalAttestationForReviewID returns the requested
// GitHub pull request approval attestation for the specified GitHub instance,
// review ID, and app. This is used when the indexPath is unknown, such as when
// dismissing a prior approval. The host information and reviewID are used to
// identify the indexPath for the requested review.
func (a *Attestations) GetGitHubPullRequestApprovalAttestationForReviewID(repo *gitinterface.Repository, hostURL string, reviewID int64, appName string) (*sslibdsse.Envelope, gitinterface.Hash, error) {
	indexPath, has, err := a.GetGitHubPullRequestApprovalIndexPathForReviewID(hostURL, reviewID)
	if err != nil {
		return nil, nil, err
	}
	if has {
		return a.GetGitHubPullRequestApprovalAttestationForIndexPath(repo, appName, indexPath)
	}

	return nil, nil, github.ErrGitHubReviewIDNotFound
}

// GetGitHubPullRequestApprovalAttestationForIndexPath returns the requested
// GitHub pull request approval attestation for the indexPath and appName.
func (a *Attestations) GetGitHubPullRequestApprovalAttestationForIndexPath(repo *gitinterface.Repository, appName, indexPath string) (*sslibdsse.Envelope, gitinterface.Hash, error) {
	// We URL encode the appName to match the on-disk path
	blobPath := path.Join(indexPath, base64.URLEncoding.EncodeToString([]byte(appName)))
	blobID, has := a.codeReviewApprovalAttestations[blobPath]
	if !has {
		return nil, nil, github.ErrPullRequestApprovalAttestationNotFound
	}

	envBytes, err := repo.ReadBlob(blobID)
	if err != nil {
		return nil, nil, err
	}

	env := &sslibdsse.Envelope{}
	if err := json.Unmarshal(envBytes, env); err != nil {
		return nil, nil, err
	}

	return env, blobID, nil
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
