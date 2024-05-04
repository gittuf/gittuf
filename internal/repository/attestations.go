// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/google/go-github/v61/github"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

var ErrNotSigningKey = errors.New("expected signing key")

var githubClient *github.Client

// AddReferenceAuthorization adds a reference authorization attestation to the
// repository for the specified target ref. The from ID is identified using the
// last RSL entry for the target ref. The to ID is that of the expected Git tree
// created by merging the feature ref into the target ref. The commit used to
// calculate the merge tree ID is identified using the RSL for the feature ref.
// Currently, this is limited to developer mode.
func (r *Repository) AddReferenceAuthorization(ctx context.Context, signer sslibdsse.SignerVerifier, targetRef, featureRef string, signCommit bool) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	var err error

	targetRef, err = r.r.AbsoluteReference(targetRef)
	if err != nil {
		return err
	}

	featureRef, err = r.r.AbsoluteReference(featureRef)
	if err != nil {
		return err
	}

	var (
		fromID          gitinterface.Hash
		featureCommitID gitinterface.Hash
		toID            gitinterface.Hash
	)

	slog.Debug("Identifying current status of target Git reference...")
	latestTargetEntry, _, err := rsl.GetLatestReferenceEntryForRef(r.r, targetRef)
	if err == nil {
		fromID = latestTargetEntry.TargetID
	} else {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return err
		}
		fromID = gitinterface.ZeroHash
	}

	slog.Debug("Identifying current status of feature Git reference...")
	latestFeatureEntry, _, err := rsl.GetLatestReferenceEntryForRef(r.r, featureRef)
	if err != nil {
		// We don't have an RSL entry for the feature ref to use to approve the
		// merge
		return err
	}
	featureCommitID = latestFeatureEntry.TargetID

	slog.Debug("Computing expected merge tree...")
	mergeTreeID, err := r.r.GetMergeTree(fromID, featureCommitID)
	if err != nil {
		return err
	}
	toID = mergeTreeID

	slog.Debug("Loading current set of attestations...")
	allAttestations, err := attestations.LoadCurrentAttestations(r.r)
	if err != nil {
		return err
	}

	// Does a reference authorization already exist for the parameters?
	hasAuthorization := false
	env, err := allAttestations.GetReferenceAuthorizationFor(r.r, targetRef, fromID.String(), toID.String())
	if err == nil {
		slog.Debug("Found existing reference authorization...")
		hasAuthorization = true
	} else if !errors.Is(err, attestations.ErrAuthorizationNotFound) {
		return err
	}

	if !hasAuthorization {
		// Create a new reference authorization and embed in env
		slog.Debug("Creating new reference authorization...")
		statement, err := attestations.NewReferenceAuthorization(targetRef, fromID.String(), toID.String())
		if err != nil {
			return err
		}

		env, err = dsse.CreateEnvelope(statement)
		if err != nil {
			return err
		}
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing reference authorization using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if err := allAttestations.SetReferenceAuthorization(r.r, env, targetRef, fromID.String(), toID.String()); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add reference authorization for '%s' from '%s' to '%s'", targetRef, fromID, toID)

	slog.Debug("Committing attestations...")
	return allAttestations.Commit(r.r, commitMessage, signCommit)
}

// RemoveReferenceAuthorization removes a previously issued authorization for
// the specified parameters. The issuer of the authorization is identified using
// their key. Currently, this is limited to developer mode.
func (r *Repository) RemoveReferenceAuthorization(ctx context.Context, signer sslibdsse.SignerVerifier, targetRef, fromID, toID string, signCommit bool) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	// Ensure only the key that created a reference authorization can remove it
	slog.Debug("Evaluating if key can sign...")
	_, err := signer.Sign(ctx, nil)
	if err != nil {
		return errors.Join(ErrNotSigningKey, err)
	}
	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	targetRef, err = r.r.AbsoluteReference(targetRef)
	if err != nil {
		return err
	}

	slog.Debug("Loading current set of attestations...")
	allAttestations, err := attestations.LoadCurrentAttestations(r.r)
	if err != nil {
		return err
	}

	slog.Debug("Loading reference authorization...")
	env, err := allAttestations.GetReferenceAuthorizationFor(r.r, targetRef, fromID, toID)
	if err != nil {
		if errors.Is(err, attestations.ErrAuthorizationNotFound) {
			// No reference authorization at all
			return nil
		}
		return err
	}

	slog.Debug("Removing signature...")
	newSignatures := []sslibdsse.Signature{}
	for _, signature := range env.Signatures {
		// This handles cases where the envelope may unintentionally have
		// multiple signatures from the same key
		if signature.KeyID != keyID {
			newSignatures = append(newSignatures, signature)
		}
	}

	if len(newSignatures) == 0 {
		// No signatures, we can remove the ReferenceAuthorization altogether
		if err := allAttestations.RemoveReferenceAuthorization(targetRef, fromID, toID); err != nil {
			return err
		}
	} else {
		// We still have other signatures, so set the ReferenceAuthorization
		// envelope
		env.Signatures = newSignatures
		if err := allAttestations.SetReferenceAuthorization(r.r, env, targetRef, fromID, toID); err != nil {
			return err
		}
	}

	commitMessage := fmt.Sprintf("Remove reference authorization for '%s' from '%s' to '%s' by '%s'", targetRef, fromID, toID, keyID)

	slog.Debug("Committing attestations...")
	return allAttestations.Commit(r.r, commitMessage, signCommit)
}

// AddGitHubPullRequestAttestationForCommit identifies the pull request for a
// specified commit ID and triggers AddGitHubPullRequestAttestationForNumber for
// that pull request. Currently, the authentication token for the GitHub API is
// read from the GITHUB_TOKEN environment variable.
func (r *Repository) AddGitHubPullRequestAttestationForCommit(ctx context.Context, signer sslibdsse.SignerVerifier, owner, repository, commitID, baseBranch string, signCommit bool) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	client := getGitHubClient()

	slog.Debug("Identifying GitHub pull requests for commit...")
	pullRequests, _, err := client.PullRequests.ListPullRequestsWithCommit(ctx, owner, repository, commitID, nil)
	if err != nil {
		return err
	}

	baseBranch, err = r.r.AbsoluteReference(baseBranch)
	if err != nil {
		return err
	}

	for _, pullRequest := range pullRequests {
		slog.Debug(fmt.Sprintf("Inspecting GitHub pull request %d...", *pullRequest.Number))
		pullRequestBranch := plumbing.NewBranchReferenceName(*pullRequest.Base.Ref).String()

		// pullRequest.Merged is not set on this endpoint for some reason
		if pullRequest.MergedAt != nil && pullRequestBranch == baseBranch {
			return r.addGitHubPullRequestAttestation(ctx, signer, owner, repository, pullRequest, signCommit)
		}
	}

	return fmt.Errorf("pull request not found for commit")
}

// AddGitHubPullRequestAttestationForNumber wraps the API response for the
// specified pull request in an in-toto attestation. `pullRequestID` must be the
// number of the pull request. Currently, the authentication token for the
// GitHub API is read from the GITHUB_TOKEN environment variable.
func (r *Repository) AddGitHubPullRequestAttestationForNumber(ctx context.Context, signer sslibdsse.SignerVerifier, owner, repository string, pullRequestNumber int, signCommit bool) error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	client := getGitHubClient()

	slog.Debug(fmt.Sprintf("Inspecting GitHub pull request %d...", pullRequestNumber))
	pullRequest, _, err := client.PullRequests.Get(ctx, owner, repository, pullRequestNumber)
	if err != nil {
		return err
	}

	return r.addGitHubPullRequestAttestation(ctx, signer, owner, repository, pullRequest, signCommit)
}

func (r *Repository) addGitHubPullRequestAttestation(ctx context.Context, signer sslibdsse.SignerVerifier, owner, repository string, pullRequest *github.PullRequest, signCommit bool) error {
	var (
		targetRef      string
		targetCommitID string
	)

	if pullRequest.MergedAt == nil {
		// not yet merged
		targetRef = fmt.Sprintf("%s-%d/refs/heads/%s", *pullRequest.Head.User.Login, *pullRequest.Head.User.ID, *pullRequest.Head.Ref)
		targetCommitID = *pullRequest.Head.SHA
	} else {
		// merged
		targetRef = fmt.Sprintf("%s-%d/refs/heads/%s", *pullRequest.Base.User.Login, *pullRequest.Base.User.ID, *pullRequest.Base.Ref)
		targetCommitID = *pullRequest.MergeCommitSHA
	}

	slog.Debug("Creating GitHub pull request attestation...")
	statement, err := attestations.NewGitHubPullRequestAttestation(owner, repository, *pullRequest.Number, targetCommitID, pullRequest)
	if err != nil {
		return err
	}

	env, err := dsse.CreateEnvelope(statement)
	if err != nil {
		return err
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing GitHub pull request attestation using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	allAttestations, err := attestations.LoadCurrentAttestations(r.r)
	if err != nil {
		return err
	}

	if err := allAttestations.SetGitHubPullRequestAuthorization(r.r, env, targetRef, targetCommitID); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add GitHub pull request attestation for '%s' at '%s'\n\nSource: https://github.com/%s/%s/pull/%d\n", targetRef, targetCommitID, owner, repository, *pullRequest.Number)

	slog.Debug("Committing attestations...")
	return allAttestations.Commit(r.r, commitMessage, signCommit)
}

func getGitHubClient() *github.Client {
	if githubClient == nil {
		githubClient = github.NewClient(nil).WithAuthToken(os.Getenv("GITHUB_TOKEN"))
	}

	return githubClient
}
