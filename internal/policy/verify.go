// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	ita "github.com/in-toto/attestation/go/v1"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

var (
	ErrUnauthorizedSignature   = errors.New("unauthorized signature")
	ErrInvalidEntryNotSkipped  = errors.New("invalid entry found not marked as skipped")
	ErrLastGoodEntryIsSkipped  = errors.New("entry expected to be unskipped is marked as skipped")
	ErrInvalidVerifier         = errors.New("verifier has invalid parameters (is threshold 0?)")
	ErrVerifierConditionsUnmet = errors.New("verifier's key and threshold constraints not met")
)

// VerifyRef verifies the signature on the latest RSL entry for the target ref
// using the latest policy. The expected Git ID for the ref in the latest RSL
// entry is returned if the policy verification is successful.
func VerifyRef(ctx context.Context, repo *gitinterface.Repository, target string) (gitinterface.Hash, error) {
	// Get latest policy entry
	slog.Debug("Loading policy...")
	policyState, err := LoadCurrentState(ctx, repo, PolicyRef)
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	// Find latest entry for target
	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", target))
	latestEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, target)
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	// Find latest set of attestations
	slog.Debug("Loading current set of attestations...")
	attestationsState, err := attestations.LoadCurrentAttestations(repo)
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	slog.Debug("Verifying entry...")
	return latestEntry.TargetID, verifyEntry(ctx, repo, policyState, attestationsState, latestEntry)
}

// VerifyRefFull verifies the entire RSL for the target ref from the first
// entry. The expected Git ID for the ref in the latest RSL entry is returned if
// the policy verification is successful.
func VerifyRefFull(ctx context.Context, repo *gitinterface.Repository, target string) (gitinterface.Hash, error) {
	// Trace RSL back to the start
	slog.Debug("Identifying first RSL entry...")
	firstEntry, _, err := rsl.GetFirstEntry(repo)
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	// Find latest entry for target
	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", target))
	latestEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, target)
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	// Do a relative verify from start entry to the latest entry (firstEntry here == policyEntry)
	// Also, attestations is initially nil because we haven't seen any yet
	slog.Debug("Verifying all entries...")
	return latestEntry.TargetID, VerifyRelativeForRef(ctx, repo, firstEntry, nil, firstEntry, latestEntry, target)
}

// VerifyRefFromEntry performs verification for the reference from a specific
// RSL entry. The expected Git ID for the ref in the latest RSL entry is
// returned if the policy verification is successful.
func VerifyRefFromEntry(ctx context.Context, repo *gitinterface.Repository, target string, entryID gitinterface.Hash) (gitinterface.Hash, error) {
	// Load starting point entry
	slog.Debug("Identifying starting RSL entry...")
	fromEntryT, err := rsl.GetEntry(repo, entryID)
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	// TODO: we should instead find the latest ref entry before the entryID and
	// use that
	fromEntry, isRefEntry := fromEntryT.(*rsl.ReferenceEntry)
	if !isRefEntry {
		return gitinterface.ZeroHash, err
	}

	// Find latest entry for target
	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", target))
	latestEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, target)
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	// Find policy entry before the starting point entry
	slog.Debug("Identifying applicable policy entry...")
	policyEntry, _, err := rsl.GetLatestReferenceEntryForRefBefore(repo, PolicyRef, fromEntry.GetID())
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	slog.Debug("Identifying applicable attestations entry...")
	var attestationsEntry *rsl.ReferenceEntry
	attestationsEntry, _, err = rsl.GetLatestReferenceEntryForRefBefore(repo, attestations.Ref, fromEntry.GetID())
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return gitinterface.ZeroHash, err
		}
	}

	// Do a relative verify from start entry to the latest entry
	slog.Debug("Verifying all entries...")
	return latestEntry.TargetID, VerifyRelativeForRef(ctx, repo, policyEntry, attestationsEntry, fromEntry, latestEntry, target)
}

// VerifyRelativeForRef verifies the RSL between specified start and end entries
// using the provided policy entry for the first entry.
//
// TODO: should the policy entry be inferred from the specified first entry?
func VerifyRelativeForRef(ctx context.Context, repo *gitinterface.Repository, initialPolicyEntry, initialAttestationsEntry, firstEntry, lastEntry *rsl.ReferenceEntry, target string) error {
	var (
		currentPolicy       *State
		currentAttestations *attestations.Attestations
	)

	// Load policy applicable at firstEntry
	slog.Debug("Loading initial policy...")
	state, err := LoadState(ctx, repo, initialPolicyEntry)
	if err != nil {
		return err
	}
	currentPolicy = state

	if initialAttestationsEntry != nil {
		slog.Debug("Loading attestations...")
		attestationsState, err := attestations.LoadAttestationsForEntry(repo, initialAttestationsEntry)
		if err != nil {
			return err
		}
		currentAttestations = attestationsState
	}

	// Enumerate RSL entries between firstEntry and lastEntry, ignoring irrelevant ones
	slog.Debug("Identifying all entries in range...")
	entries, annotations, err := rsl.GetReferenceEntriesInRangeForRef(repo, firstEntry.ID, lastEntry.ID, target)
	if err != nil {
		return err
	}

	// Verify each entry, looking for a fix when an invalid entry is encountered
	var invalidEntry *rsl.ReferenceEntry
	var verificationErr error
	for len(entries) != 0 {
		if invalidEntry == nil {
			// Pop entry from queue
			entry := entries[0]
			entries = entries[1:]

			slog.Debug(fmt.Sprintf("Verifying entry '%s'...", entry.ID.String()))

			slog.Debug("Checking if entry is for policy staging reference...")
			if entry.RefName == PolicyStagingRef {
				continue
			}
			slog.Debug("Checking if entry is for policy reference...")
			if entry.RefName == PolicyRef {
				// TODO: this is repetition if the firstEntry is for policy
				newPolicy, err := loadStateForEntry(repo, entry)
				if err != nil {
					return err
				}

				slog.Debug("Verifying new policy using current policy...")
				if err := currentPolicy.VerifyNewState(ctx, newPolicy); err != nil {
					return err
				}

				slog.Debug("Updating current policy...")
				currentPolicy = newPolicy
				continue
			}

			slog.Debug("Checking if entry is for attestations reference...")
			if entry.RefName == attestations.Ref {
				newAttestationsState, err := attestations.LoadAttestationsForEntry(repo, entry)
				if err != nil {
					return err
				}

				currentAttestations = newAttestationsState
				continue
			}

			slog.Debug("Verifying changes...")
			if err := verifyEntry(ctx, repo, currentPolicy, currentAttestations, entry); err != nil {
				slog.Debug("Violation found, checking if entry has been revoked...")
				// If the invalid entry is never marked as skipped, we return err
				if !entry.SkippedBy(annotations[entry.ID.String()]) {
					return err
				}

				// The invalid entry's been marked as skipped but we still need
				// to see if another entry fixed state for non-gittuf users
				slog.Debug("Entry has been revoked, searching for fix entry...")
				invalidEntry = entry
				verificationErr = err

				if len(entries) == 0 {
					// Fix entry does not exist after revoking annotation
					return verificationErr
				}
			}
			continue
		}

		// This is only reached when we have an invalid state.
		// First, the verification workflow determines the last good state for
		// the ref. This is needed to evaluate whether a fix for the invalid
		// state is available. After this is found, the workflow looks through
		// the remaining entries in the queue to find the fix. Until the fix is
		// found, entries encountered that are for other refs are added to a new
		// queue. Entries that are for the same ref but not the fix are
		// considered invalid. The workflow enters a valid state again when a)
		// the fix entry (which hasn't also been revoked) is found, and b) all
		// entries for the ref in the invalid range are marked as skipped by an
		// annotation. If these conditions don't both hold, the workflow returns
		// an error. After the fix is found, all remaining entries in the
		// original queue are also added to the new queue. The new queue then
		// takes the place of the original queue. This ensures that all entries
		// are processed even when an invalid state is reached.

		// 1. What's the last good state?
		slog.Debug("Identifying last valid state...")
		lastGoodEntry, lastGoodEntryAnnotations, err := rsl.GetLatestUnskippedReferenceEntryForRefBefore(repo, invalidEntry.RefName, invalidEntry.ID)
		if err != nil {
			return err
		}
		slog.Debug("Verifying identified last valid entry has not been revoked...")
		if lastGoodEntry.SkippedBy(lastGoodEntryAnnotations) {
			return ErrLastGoodEntryIsSkipped
		}
		// gittuf requires the fix to point to a commit that is tree-same as the
		// last good state
		lastGoodTreeID, err := repo.GetCommitTreeID(lastGoodEntry.TargetID)
		if err != nil {
			return err
		}

		// 2. What entries do we have in the current verification set for the
		// ref? The first one that is tree-same as lastGoodEntry's commit is the
		// fix. Entries prior to that one in the queue are considered invalid
		// and must be skipped
		fixed := false
		invalidIntermediateEntries := []*rsl.ReferenceEntry{}
		newEntryQueue := []*rsl.ReferenceEntry{}
		for len(entries) != 0 {
			newEntry := entries[0]
			entries = entries[1:]

			slog.Debug(fmt.Sprintf("Inspecting entry '%s' to see if it's a fix entry...", newEntry.ID.String()))

			slog.Debug("Checking if entry is for the affected reference...")
			if newEntry.RefName != invalidEntry.RefName {
				// Unrelated entry that must be processed in the outer loop
				// Currently this is just policy entries
				newEntryQueue = append(newEntryQueue, newEntry)
				continue
			}

			newCommitTreeID, err := repo.GetCommitTreeID(newEntry.TargetID)
			if err != nil {
				return err
			}

			slog.Debug("Checking if entry is tree-same with last valid state...")
			if newCommitTreeID.Equal(lastGoodTreeID) {
				// Fix found, we append the rest of the current verification set
				// to the new entry queue
				// But first, we must check that this fix hasn't been skipped
				// If it has been skipped, it's not actually a fix and we need
				// to keep looking
				slog.Debug("Verifying potential fix entry has not been revoked...")
				if !newEntry.SkippedBy(annotations[newEntry.ID.String()]) {
					slog.Debug("Fix entry found, proceeding with regular verification workflow...")
					fixed = true
					newEntryQueue = append(newEntryQueue, entries...)
					break
				}
			}

			// newEntry is not tree-same / commit-same, so it is automatically
			// invalid, check that it's been marked as revoked
			slog.Debug("Checking non-fix entry has been revoked as well...")
			if !newEntry.SkippedBy(annotations[newEntry.ID.String()]) {
				invalidIntermediateEntries = append(invalidIntermediateEntries, newEntry)
			}
		}

		if !fixed {
			// If we haven't found a fix, return the original error
			return verificationErr
		}

		if len(invalidIntermediateEntries) != 0 {
			// We may have found a fix but if an invalid intermediate entry
			// wasn't skipped, return error
			return ErrInvalidEntryNotSkipped
		}

		// Reset these trackers to continue verification with rest of the queue
		// We may encounter other issues
		invalidEntry = nil
		verificationErr = nil

		entries = newEntryQueue
	}

	return nil
}

// VerifyNewState ensures that when a new policy is encountered, its root role
// is signed by keys trusted in the current policy.
func (s *State) VerifyNewState(ctx context.Context, newPolicy *State) error {
	rootVerifier, err := s.getRootVerifier()
	if err != nil {
		return err
	}

	_, err = rootVerifier.Verify(ctx, gitinterface.ZeroHash, newPolicy.RootEnvelope)
	return err
}

// verifyEntry is a helper to verify an entry's signature using the specified
// policy. The specified policy is used for the RSL entry itself. However, for
// commit signatures, verifyEntry checks when the commit was first introduced
// via the RSL across all refs. Then, it uses the policy applicable at the
// commit's first entry into the repository. If the commit is brand new to the
// repository, the specified policy is used.
func verifyEntry(ctx context.Context, repo *gitinterface.Repository, policy *State, attestationsState *attestations.Attestations, entry *rsl.ReferenceEntry) error {
	if entry.RefName == PolicyRef || entry.RefName == attestations.Ref {
		return nil
	}

	if strings.HasPrefix(entry.RefName, gitinterface.TagRefPrefix) {
		return verifyTagEntry(ctx, repo, policy, attestationsState, entry)
	}

	// Load the applicable reference authorization and approvals from trusted
	// code review systems
	authorizationAttestation, approverKeyIDs, err := getApproverAttestationAndKeyIDs(ctx, repo, policy, attestationsState, entry)
	if err != nil {
		return err
	}

	// Verify Git namespace policies using the RSL entry and attestations
	if _, err := verifyGitObjectAndAttestations(ctx, policy, fmt.Sprintf("%s:%s", gitReferenceRuleScheme, entry.RefName), entry.ID, authorizationAttestation, approverKeyIDs); err != nil {
		return fmt.Errorf("verifying Git namespace policies failed, %w", ErrUnauthorizedSignature)
	}

	// Check if policy has file rules at all for efficiency
	hasFileRule, err := policy.hasFileRule()
	if err != nil {
		return err
	}

	if !hasFileRule {
		// No file rules to verify
		return nil
	}

	// Verify modified files

	// First, get all commits between the current and last entry for the ref.
	commitIDs, err := getCommits(repo, entry) // note: this is ordered by commit ID
	if err != nil {
		return err
	}

	for _, commitID := range commitIDs {
		paths, err := repo.GetFilePathsChangedByCommit(commitID)
		if err != nil {
			return err
		}

		verifiedUsing := "" // this will be set after one successful verification of the commit to avoid repeated signature verification
		for _, path := range paths {
			verifiers, err := policy.FindVerifiersForPath(fmt.Sprintf("%s:%s", fileRuleScheme, path))
			if err != nil {
				return err
			}

			verified := false
			if len(verifiedUsing) > 0 {
				// We've already verified and identified commit signature, we
				// can just check if that verifier is trusted for the new path.
				// If not found, we don't make any assumptions about it being a
				// failure in case of name mismatches. So, the signature check
				// proceeds as usual.
				for _, verifier := range verifiers {
					if verifier.Name() == verifiedUsing {
						verified = true
						break
					}
				}
			}

			if verified {
				continue
			}

			verifiedUsing, err = verifyGitObjectAndAttestationsUsingVerifiers(ctx, verifiers, commitID, authorizationAttestation, approverKeyIDs)
			if err != nil {
				return fmt.Errorf("verifying file namespace policies failed, %w", ErrUnauthorizedSignature)
			}
		}
	}

	return nil
}

func verifyTagEntry(ctx context.Context, repo *gitinterface.Repository, policy *State, attestationsState *attestations.Attestations, entry *rsl.ReferenceEntry) error {
	entryTagRef, err := repo.GetReference(entry.RefName)
	if err != nil {
		return err
	}

	tagTargetID, err := repo.GetTagTarget(entry.TargetID)
	if err != nil {
		return err
	}

	if !entry.TargetID.Equal(entryTagRef) && !entry.TargetID.Equal(tagTargetID) {
		return fmt.Errorf("verifying RSL entry failed, tag reference set to unexpected target")
	}

	authorizationAttestation, approverKeyIDs, err := getApproverAttestationAndKeyIDs(ctx, repo, policy, attestationsState, entry)
	if err != nil {
		return err
	}

	// Find authorized verifiers for tag's RSL entry
	verifiers, err := policy.FindVerifiersForPath(fmt.Sprintf("%s:%s", gitReferenceRuleScheme, entry.RefName))
	if err != nil {
		return err
	}

	if len(verifiers) == 0 {
		return nil
	}

	// Use each verifier to verify signature
	if _, err := verifyGitObjectAndAttestationsUsingVerifiers(ctx, verifiers, entry.ID, authorizationAttestation, approverKeyIDs); err != nil {
		return fmt.Errorf("verifying RSL entry failed, %w", ErrUnauthorizedSignature)
	}

	// Verify tag object
	tagObjVerified := false
	for _, verifier := range verifiers {
		// explicitly not looking at the attestation
		// that applies to the _push_
		// thus, we also set threshold to 1
		verifier.threshold = 1

		_, err := verifier.Verify(ctx, entry.TargetID, nil)
		if err == nil {
			// Signature verification succeeded
			tagObjVerified = true
			break
		} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
			// Unexpected error
			return err
		}
		// Haven't found a valid verifier, continue with next verifier
	}

	if !tagObjVerified {
		return fmt.Errorf("verifying tag object's signature failed, %w", ErrUnauthorizedSignature)
	}

	return nil
}

func getApproverAttestationAndKeyIDs(ctx context.Context, repo *gitinterface.Repository, policy *State, attestationsState *attestations.Attestations, entry *rsl.ReferenceEntry) (*sslibdsse.Envelope, *set.Set[string], error) {
	if attestationsState == nil {
		return nil, nil, nil
	}

	firstEntry := false
	priorRefEntry, _, err := rsl.GetLatestReferenceEntryForRefBefore(repo, entry.RefName, entry.ID)
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil, nil, err
		}

		firstEntry = true
	}

	fromID := gitinterface.ZeroHash
	if !firstEntry {
		fromID = priorRefEntry.TargetID
	}

	entryTreeID, err := repo.GetCommitTreeID(entry.TargetID)
	if err != nil {
		return nil, nil, err
	}

	authorizationAttestation, err := attestationsState.GetReferenceAuthorizationFor(repo, entry.RefName, fromID.String(), entryTreeID.String())
	if err != nil {
		if !errors.Is(err, attestations.ErrAuthorizationNotFound) {
			return nil, nil, err
		}
	}

	approverKeyIDs := set.NewSet[string]()

	// When we add other code review systems, we can move this into a
	// generalized helper that inspects the attestations for each system trusted
	// in policy.
	if policy.githubAppApprovalsTrusted {
		githubApprovalAttestation, err := attestationsState.GetGitHubPullRequestApprovalAttestationFor(repo, entry.RefName, fromID.String(), entryTreeID.String())
		if err != nil {
			if !errors.Is(err, attestations.ErrGitHubPullRequestApprovalAttestationNotFound) {
				return nil, nil, err
			}
		}

		// if it exists
		if githubApprovalAttestation != nil {
			approvalVerifier := &Verifier{
				repository: policy.repository,
				name:       GitHubAppRoleName,
				keys:       []*tuf.Key{policy.githubAppKey},
				threshold:  1,
			}
			_, err := approvalVerifier.Verify(ctx, nil, githubApprovalAttestation)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to verify GitHub app approval attestation, signed by untrusted key")
			}

			payloadBytes, err := githubApprovalAttestation.DecodeB64Payload()
			if err != nil {
				return nil, nil, err
			}

			type tmpStatement struct {
				Type          string                                             `json:"_type"`
				Subject       []*ita.ResourceDescriptor                          `json:"subject"`
				PredicateType string                                             `json:"predicateType"`
				Predicate     *attestations.GitHubPullRequestApprovalAttestation `json:"predicate"`
			}
			stmt := new(tmpStatement)
			if err := json.Unmarshal(payloadBytes, stmt); err != nil {
				return nil, nil, err
			}

			for _, approver := range stmt.Predicate.Approvers {
				approverKeyIDs.Add(approver.KeyID)
			}
		}
	}

	return authorizationAttestation, approverKeyIDs, nil
}

// getCommits identifies the commits introduced to the entry's ref since the
// last RSL entry for the same ref. These commits are then verified for file
// policies.
func getCommits(repo *gitinterface.Repository, entry *rsl.ReferenceEntry) ([]gitinterface.Hash, error) {
	firstEntry := false

	priorRefEntry, _, err := rsl.GetLatestReferenceEntryForRefBefore(repo, entry.RefName, entry.ID)
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil, err
		}

		firstEntry = true
	}

	if firstEntry {
		return repo.GetCommitsBetweenRange(entry.TargetID, gitinterface.ZeroHash)
	}

	return repo.GetCommitsBetweenRange(entry.TargetID, priorRefEntry.TargetID)
}

func verifyGitObjectAndAttestations(ctx context.Context, policy *State, target string, gitID gitinterface.Hash, authorizationAttestation *sslibdsse.Envelope, approverKeyIDs *set.Set[string]) (string, error) {
	verifiers, err := policy.FindVerifiersForPath(target)
	if err != nil {
		return "", err
	}

	return verifyGitObjectAndAttestationsUsingVerifiers(ctx, verifiers, gitID, authorizationAttestation, approverKeyIDs)
}

func verifyGitObjectAndAttestationsUsingVerifiers(ctx context.Context, verifiers []*Verifier, gitID gitinterface.Hash, authorizationAttestation *sslibdsse.Envelope, approverKeyIDs *set.Set[string]) (string, error) {
	if len(verifiers) == 0 {
		// This target is not protected by gittuf policy
		return "", nil
	}

	verifiedUsing := ""
	for _, verifier := range verifiers {
		trustedKeyIDs := verifier.TrustedKeyIDs()

		usedKeyIDs, err := verifier.Verify(ctx, gitID, authorizationAttestation)
		if err == nil {
			// We meet requirements just from the authorization attestation's sigs
			verifiedUsing = verifier.Name()
			break
		} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
			return "", err
		}

		// Unify the keyIDs we've already used with that listed in approval attestation
		// We ensure that someone who has signed an attestation and is listed in
		// the approval attestation is only counted once
		usedKeyIDs.Extend(approverKeyIDs)

		// Get a list of used keys that are also trusted by the verifier
		trustedUsedKeyIDs := trustedKeyIDs.Intersection(usedKeyIDs)
		if trustedUsedKeyIDs.Len() >= verifier.Threshold() {
			// With approvals, we now meet threshold!
			verifiedUsing = verifier.Name()
			break
		}
	}

	if verifiedUsing != "" {
		return verifiedUsing, nil
	}

	return "", ErrVerifierConditionsUnmet
}

type Verifier struct {
	repository *gitinterface.Repository
	name       string
	keys       []*tuf.Key
	threshold  int
}

func (v *Verifier) Name() string {
	return v.name
}

func (v *Verifier) Keys() []*tuf.Key {
	return v.keys
}

func (v *Verifier) Threshold() int {
	return v.threshold
}

func (v *Verifier) TrustedKeyIDs() *set.Set[string] {
	keys := set.NewSet[string]()
	for _, key := range v.keys {
		keys.Add(key.KeyID)
	}

	return keys
}

// Verify is used to check for a threshold of signatures using the verifier. The
// threshold of signatures may be met using a combination of at most one Git
// signature and signatures embedded in a DSSE envelope. Verify does not inspect
// the envelope's payload, but instead only verifies the signatures. The caller
// must ensure the validity of the envelope's contents.
func (v *Verifier) Verify(ctx context.Context, gitObjectID gitinterface.Hash, env *sslibdsse.Envelope) (*set.Set[string], error) {
	if v.threshold < 1 || len(v.keys) < 1 {
		return nil, ErrInvalidVerifier
	}

	usedKeyIDs := set.NewSet[string]()
	gitObjectVerified := false

	// First, verify the gitObject's signature if one is presented
	if gitObjectID != nil && !gitObjectID.IsZero() {
		for _, key := range v.keys {
			err := v.repository.VerifySignature(ctx, gitObjectID, key)
			if err == nil {
				// Signature verification succeeded
				usedKeyIDs.Add(key.KeyID)
				gitObjectVerified = true
				break
			}
			if errors.Is(err, gitinterface.ErrUnknownSigningMethod) {
				continue
			}
			if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
				return nil, err
			}
		}
	}

	// If threshold is 1 and the Git signature is verified, we can return
	if v.threshold == 1 && gitObjectVerified {
		return usedKeyIDs, nil
	}

	if env != nil {
		// Second, verify signatures on the attestation, subtracting the threshold
		// by 1 to account for a verified Git signature
		envelopeThreshold := v.threshold
		if gitObjectVerified {
			envelopeThreshold--
		}

		envVerifiers := make([]sslibdsse.Verifier, 0, len(v.keys))
		for _, key := range v.keys {
			if usedKeyIDs.Has(key.KeyID) {
				// Do not create a DSSE verifier for the key used to verify the Git
				// signature
				continue
			}

			verifier, err := signerverifier.NewSignerVerifierFromTUFKey(key) //nolint:staticcheck
			if err != nil && !errors.Is(err, common.ErrUnknownKeyType) {
				return nil, err
			}
			envVerifiers = append(envVerifiers, verifier)
		}

		acceptedKeys, err := dsse.VerifyEnvelope(ctx, env, envVerifiers, envelopeThreshold)
		if err != nil && !strings.Contains(err.Error(), "accepted signatures do not match threshold") {
			return nil, err
		}
		for _, ak := range acceptedKeys {
			usedKeyIDs.Add(ak.KeyID)
		}

		if usedKeyIDs.Len() >= v.Threshold() {
			return usedKeyIDs, nil
		}
	}

	// Return usedKeyIDs so the consumer can decide what to do with the keys
	// that were used
	return usedKeyIDs, ErrVerifierConditionsUnmet
}
