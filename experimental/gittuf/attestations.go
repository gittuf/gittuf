// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"strings"

	attestopts "github.com/gittuf/gittuf/experimental/gittuf/options/attest"
	githubopts "github.com/gittuf/gittuf/experimental/gittuf/options/github"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/attestations/authorizations"
	"github.com/gittuf/gittuf/internal/attestations/github"
	githubv01 "github.com/gittuf/gittuf/internal/attestations/github/v01"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5/plumbing"
	gogithub "github.com/google/go-github/v61/github"
	ita "github.com/in-toto/attestation/go/v1"
)

const githubTokenEnvKey = "GITHUB_TOKEN" //nolint:gosec

var (
	ErrNotSigningKey = errors.New("expected signing key")
	ErrNoGitHubToken = errors.New("authentication token for GitHub API not provided")
)

var githubClient *gogithub.Client

// ApplyAttestations records the state of the attestations reference and syncs
// it with the specified remote, making the attestation available at the
// synchronization point.
func (r *Repository) ApplyAttestations(ctx context.Context, remoteName string, localOnly, signRSLEntry bool) error {
	if signRSLEntry {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	opts := []rslopts.RecordOption{rslopts.WithRecordRemote(remoteName)}
	if localOnly {
		opts = append(opts, rslopts.WithRecordLocalOnly())
	}

	return r.RecordRSLEntryForReference(ctx, attestations.Ref, signRSLEntry, opts...)
}

// AddReferenceAuthorization adds a reference authorization attestation to the
// repository for the specified target ref. The from ID is identified using the
// last RSL entry for the target ref. The to ID is that of the expected Git tree
// created by merging the feature ref into the target ref. The commit used to
// calculate the merge tree ID is identified using the RSL for the feature ref.
func (r *Repository) AddReferenceAuthorization(ctx context.Context, signer sslibdsse.SignerVerifier, targetRef, featureRef string, signCommit bool, opts ...attestopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &attestopts.Options{}
	for _, fn := range opts {
		fn(options)
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

	isTag := strings.HasPrefix(targetRef, gitinterface.TagRefPrefix)

	slog.Debug("Identifying current status of target Git reference...")
	latestTargetEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.r, rsl.ForReference(targetRef))
	if err == nil {
		if isTag {
			return fmt.Errorf("cannot approve a tag that already exists: %w", gitinterface.ErrTagAlreadyExists)
		}

		fromID = latestTargetEntry.GetTargetID()
	} else {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return err
		}
		fromID = gitinterface.ZeroHash
	}

	slog.Debug("Identifying current status of feature Git reference...")
	latestFeatureEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(r.r, rsl.ForReference(featureRef))
	if err != nil {
		// We don't have an RSL entry for the feature ref to use to approve the
		// merge
		return err
	}
	featureCommitID = latestFeatureEntry.GetTargetID()

	if isTag {
		// for tags, the toID is the commitID the tag will point to
		toID = featureCommitID
	} else {
		slog.Debug("Computing expected merge tree...")
		mergeTreeID, err := r.r.GetMergeTree(fromID, featureCommitID)
		if err != nil {
			return err
		}
		toID = mergeTreeID
	}

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
	} else if !errors.Is(err, authorizations.ErrAuthorizationNotFound) {
		return err
	}

	if !hasAuthorization {
		// Create a new reference authorization and embed in env
		slog.Debug("Creating new reference authorization...")
		var statement *ita.Statement
		if isTag {
			statement, err = attestations.NewReferenceAuthorizationForTag(targetRef, fromID.String(), toID.String())
		} else {
			statement, err = attestations.NewReferenceAuthorizationForCommit(targetRef, fromID.String(), toID.String())
		}
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
	if isTag {
		commitMessage = fmt.Sprintf("Add reference authorization for '%s' at '%s'", targetRef, toID.String())
	}

	slog.Debug("Committing attestations...")
	return allAttestations.Commit(r.r, commitMessage, options.CreateRSLEntry, signCommit)
}

// RemoveReferenceAuthorization removes a previously issued authorization for
// the specified parameters. The issuer of the authorization is identified using
// their key.
func (r *Repository) RemoveReferenceAuthorization(ctx context.Context, signer sslibdsse.SignerVerifier, targetRef, fromID, toID string, signCommit bool, opts ...attestopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := &attestopts.Options{}
	for _, fn := range opts {
		fn(options)
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
		if errors.Is(err, authorizations.ErrAuthorizationNotFound) {
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
	return allAttestations.Commit(r.r, commitMessage, options.CreateRSLEntry, signCommit)
}

// AddGitHubPullRequestAttestationForCommit identifies the pull request for a
// specified commit ID and triggers AddGitHubPullRequestAttestationForNumber for
// that pull request. The authentication token for the GitHub API can be passed
// in as an option. If it is not, it is read from the GITHUB_TOKEN environment
// variable. A custom GitHub instance can be specified via opts.
func (r *Repository) AddGitHubPullRequestAttestationForCommit(ctx context.Context, signer sslibdsse.SignerVerifier, owner, repository, commitID, baseBranch string, signCommit bool, opts ...githubopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := githubopts.DefaultOptions
	for _, fn := range opts {
		fn(options)
	}

	if options.GitHubToken == "" {
		options.GitHubToken = os.Getenv(githubTokenEnvKey)

		if options.GitHubToken == "" {
			// still empty
			return ErrNoGitHubToken
		}
	}

	client, err := getGitHubClient(options.GitHubBaseURL, options.GitHubToken)
	if err != nil {
		return err
	}

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
			return r.addGitHubPullRequestAttestation(ctx, signer, options.GitHubBaseURL, owner, repository, pullRequest, options.CreateRSLEntry, signCommit)
		}
	}

	return fmt.Errorf("pull request not found for commit")
}

// AddGitHubPullRequestAttestationForNumber wraps the API response for the
// specified pull request in an in-toto attestation. `pullRequestID` must be the
// number of the pull request. The authentication token for the GitHub API can
// be passed in as an option. If it is not passed in, it is read from the
// GITHUB_TOKEN environment variable. A custom GitHub instance can be specified
// via opts.
func (r *Repository) AddGitHubPullRequestAttestationForNumber(ctx context.Context, signer sslibdsse.SignerVerifier, owner, repository string, pullRequestNumber int, signCommit bool, opts ...githubopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := githubopts.DefaultOptions
	for _, fn := range opts {
		fn(options)
	}

	if options.GitHubToken == "" {
		options.GitHubToken = os.Getenv(githubTokenEnvKey)

		if options.GitHubToken == "" {
			// still empty
			return ErrNoGitHubToken
		}
	}

	client, err := getGitHubClient(options.GitHubBaseURL, options.GitHubToken)

	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Inspecting GitHub pull request %d...", pullRequestNumber))
	pullRequest, _, err := client.PullRequests.Get(ctx, owner, repository, pullRequestNumber)
	if err != nil {
		return err
	}

	return r.addGitHubPullRequestAttestation(ctx, signer, options.GitHubBaseURL, owner, repository, pullRequest, options.CreateRSLEntry, signCommit)
}

// AddGitHubPullRequestApprover adds a GitHub pull request approval attestation
// for the specified parameters. If an attestation already exists, the specified
// approver is added to the existing attestation's predicate and it is re-signed
// and stored in the repository. To find the review information, the GitHub API
// is used and the authentication token for the API is passed in as an option.
// If the token is not passed in, it's read from the GITHUB_TOKEN environment
// variable. A custom GitHub instance can be specified via opts.
func (r *Repository) AddGitHubPullRequestApprover(ctx context.Context, signer sslibdsse.SignerVerifier, owner, repository string, pullRequestNumber int, reviewID int64, approver string, signCommit bool, opts ...githubopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := githubopts.DefaultOptions
	for _, fn := range opts {
		fn(options)
	}

	if options.GitHubToken == "" {
		options.GitHubToken = os.Getenv(githubTokenEnvKey)

		if options.GitHubToken == "" {
			// still empty
			return ErrNoGitHubToken
		}
	}

	currentAttestations, err := attestations.LoadCurrentAttestations(r.r)
	if err != nil {
		return err
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}
	appName := tuf.GitHubAppRoleName // TODO: make this configurable, check appName's key matches signer

	baseRef, fromID, toID, err := getGitHubPullRequestReviewDetails(ctx, currentAttestations, options.GitHubBaseURL, options.GitHubToken, owner, repository, pullRequestNumber, reviewID)
	if err != nil {
		return err
	}

	// TODO: if the helper above has an indexPath, we can directly load that blob, simplifying the logic here
	hasApprovalAttestation := false
	env, err := currentAttestations.GetGitHubPullRequestApprovalAttestationFor(r.r, appName, baseRef, fromID, toID)
	if err == nil {
		slog.Debug("Found existing GitHub pull request approval attestation...")
		hasApprovalAttestation = true
	} else if !errors.Is(err, github.ErrPullRequestApprovalAttestationNotFound) {
		return err
	}

	approvers := []string{approver}
	var dismissedApprovers []string
	if !hasApprovalAttestation {
		// Create a new GitHub pull request approval attestation
		slog.Debug("Creating new GitHub pull request approval attestation...")
	} else {
		// Update existing statement's predicate and create new env
		slog.Debug("Adding approver to existing GitHub pull request approval attestation...")
		predicate, err := getGitHubPullRequestApprovalPredicateFromEnvelope(env)
		if err != nil {
			return err
		}

		approvers = append(approvers, predicate.GetApprovers()...)
		dismissedApprovers = predicate.GetDismissedApprovers()
	}

	statement, err := attestations.NewGitHubPullRequestApprovalAttestation(baseRef, fromID, toID, approvers, dismissedApprovers)
	if err != nil {
		return err
	}

	env, err = dsse.CreateEnvelope(statement)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing GitHub pull request approval attestation using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(r.r, env, options.GitHubBaseURL, reviewID, appName, baseRef, fromID, toID); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Add GitHub pull request approval for '%s' from '%s' to '%s' (review ID %d) for approval by '%s'", baseRef, fromID, toID, reviewID, approver)

	slog.Debug("Committing attestations...")
	return currentAttestations.Commit(r.r, commitMessage, options.CreateRSLEntry, signCommit)
}

// DismissGitHubPullRequestApprover removes an approver from the GitHub pull
// request approval attestation for the specified parameters. A custom GitHub
// instance can be specified via opts.
func (r *Repository) DismissGitHubPullRequestApprover(ctx context.Context, signer sslibdsse.SignerVerifier, reviewID int64, dismissedApprover string, signCommit bool, opts ...githubopts.Option) error {
	if signCommit {
		slog.Debug("Checking if Git signing is configured...")
		err := r.r.CanSign()
		if err != nil {
			return err
		}
	}

	options := githubopts.DefaultOptions
	for _, fn := range opts {
		fn(options)
	}
	// We don't hit the GitHub API for this flow, so no need to check the token
	// option

	currentAttestations, err := attestations.LoadCurrentAttestations(r.r)
	if err != nil {
		return err
	}

	keyID, err := signer.KeyID()
	if err != nil {
		return err
	}
	appName := tuf.GitHubAppRoleName

	env, err := currentAttestations.GetGitHubPullRequestApprovalAttestationForReviewID(r.r, options.GitHubBaseURL, reviewID, appName)
	if err != nil {
		return err
	}

	// Update existing statement's predicate and create new env
	slog.Debug("Updating existing GitHub pull request approval attestation...")

	predicate, err := getGitHubPullRequestApprovalPredicateFromEnvelope(env)
	if err != nil {
		return err
	}

	dismissedApprovers := []string{dismissedApprover}
	dismissedApprovers = append(dismissedApprovers, predicate.GetDismissedApprovers()...)

	approvers := make([]string, 0, len(predicate.GetApprovers()))
	for _, approver := range predicate.GetApprovers() {
		approver := approver
		if approver == dismissedApprover {
			continue
		}
		approvers = append(approvers, approver)
	}

	baseRef := predicate.GetRef()
	fromID := predicate.GetFromID()
	toID := predicate.GetTargetID()

	statement, err := attestations.NewGitHubPullRequestApprovalAttestation(baseRef, fromID, toID, approvers, dismissedApprovers)
	if err != nil {
		return err
	}

	env, err = dsse.CreateEnvelope(statement)
	if err != nil {
		return err
	}

	slog.Debug(fmt.Sprintf("Signing GitHub pull request approval attestation using '%s'...", keyID))
	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		return err
	}

	if err := currentAttestations.SetGitHubPullRequestApprovalAttestation(r.r, env, options.GitHubBaseURL, reviewID, appName, baseRef, fromID, toID); err != nil {
		return err
	}

	commitMessage := fmt.Sprintf("Dismiss GitHub pull request approval for '%s' from '%s' to '%s' (review ID %d) for approval by '%s'", baseRef, fromID, toID, reviewID, dismissedApprover)

	slog.Debug("Committing attestations...")
	return currentAttestations.Commit(r.r, commitMessage, options.CreateRSLEntry, signCommit)
}

func (r *Repository) addGitHubPullRequestAttestation(ctx context.Context, signer sslibdsse.SignerVerifier, githubBaseURL, owner, repository string, pullRequest *gogithub.PullRequest, createRSLEntry, signCommit bool) error {
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

	commitMessage := fmt.Sprintf("Add GitHub pull request attestation for '%s' at '%s'\n\nSource: %s/%s/%s/pull/%d\n", targetRef, targetCommitID, strings.TrimSuffix(githubBaseURL, "/"), owner, repository, *pullRequest.Number)

	slog.Debug("Committing attestations...")
	return allAttestations.Commit(r.r, commitMessage, createRSLEntry, signCommit)
}

func getGitHubPullRequestApprovalPredicateFromEnvelope(env *sslibdsse.Envelope) (github.PullRequestApprovalAttestation, error) {
	payloadBytes, err := env.DecodeB64Payload()
	if err != nil {
		return nil, err
	}

	// TODO: support multiple versions here

	// tmpGitHubPullRequestApprovalStatement is essentially a definition of
	// in-toto's v1 Statement. The difference is that we fix the predicate to be
	// the GitHub pull request approval type, making unmarshalling easier.
	type tmpGitHubPullRequestApprovalStatement struct {
		Type          string                                    `json:"_type"`
		Subject       []*ita.ResourceDescriptor                 `json:"subject"`
		PredicateType string                                    `json:"predicateType"`
		Predicate     *githubv01.PullRequestApprovalAttestation `json:"predicate"`
	}

	stmt := new(tmpGitHubPullRequestApprovalStatement)
	if err := json.Unmarshal(payloadBytes, stmt); err != nil {
		return nil, err
	}

	return stmt.Predicate, nil
}

func indexPathToComponents(indexPath string) (string, string, string) {
	components := strings.Split(indexPath, "/")

	fromTo := strings.Split(components[len(components)-2], "-")
	components = components[:len(components)-2] // remove last two items which are from-to and system

	base := strings.Join(components, "/") // reconstruct ref
	from := fromTo[0]
	to := fromTo[1]

	return base, from, to
}

func getGitHubPullRequestReviewDetails(ctx context.Context, currentAttestations *attestations.Attestations, githubBaseURL, githubToken, owner, repository string, pullRequestNumber int, reviewID int64) (string, string, string, error) {
	indexPath, has, err := currentAttestations.GetGitHubPullRequestApprovalIndexPathForReviewID(githubBaseURL, reviewID)
	if err != nil {
		return "", "", "", err
	}
	if has {
		base, from, to := indexPathToComponents(indexPath)
		return base, from, to, nil
	}

	// Compute details for review, this is when the review is first created as
	// other times we use the existing indexPath for the reviewID
	// Note: there's the potential for a TOCTOU issue here, we may query the
	// repo after things have moved in either branch.

	client, err := getGitHubClient(githubBaseURL, githubToken)
	if err != nil {
		return "", "", "", err
	}

	pullRequest, _, err := client.PullRequests.Get(ctx, owner, repository, pullRequestNumber)
	if err != nil {
		return "", "", "", err
	}

	if _, _, err := client.PullRequests.GetReview(ctx, owner, repository, pullRequestNumber, reviewID); err != nil {
		// testing validity of reviewID for the pull request in question
		return "", "", "", err
	}

	baseRef := gitinterface.BranchReferenceName(*pullRequest.Base.Ref)

	referenceDetails, _, err := client.Git.GetRef(ctx, owner, repository, baseRef)
	if err != nil {
		return "", "", "", err
	}
	fromID := *referenceDetails.Object.SHA // current tip of base ref

	// GitHub has already computed a merge commit, use that tree ID as target
	// tree ID
	commit, _, err := client.Git.GetCommit(ctx, owner, repository, *pullRequest.MergeCommitSHA)
	if err != nil {
		return "", "", "", err
	}
	toID := *commit.Tree.SHA

	return baseRef, fromID, toID, nil
}

// getGitHubClient creates a client to interact with a GitHub instance. If a
// base URL other than https://github.com is supplied, the client is configured
// to interact with the specified enterprise instance.
func getGitHubClient(baseURL, githubToken string) (*gogithub.Client, error) {
	if githubClient == nil {
		githubClient = gogithub.NewClient(nil).WithAuthToken(githubToken)
	}

	if baseURL != githubopts.DefaultGitHubBaseURL {
		baseURL = strings.TrimSuffix(baseURL, "/")

		endpointAPI := fmt.Sprintf("%s/%s/%s/", baseURL, "api", "v3")
		endpointUpload := fmt.Sprintf("%s/%s/%s/", baseURL, "api", "uploads")

		var err error
		githubClient, err = githubClient.WithEnterpriseURLs(endpointAPI, endpointUpload)
		if err != nil {
			return nil, err
		}
	}

	return githubClient, nil
}
