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

func VerifyMergeable(ctx context.Context, repo *gitinterface.Repository, targetRef, featureRef string) (bool, error) {
	slog.Debug("Loading policy...")
	policyState, err := LoadCurrentState(ctx, repo, PolicyRef)
	if err != nil {
		return false, err
	}

	slog.Debug("Loading current set of attestations...")
	attestationsState, err := attestations.LoadCurrentAttestations(repo)
	if err != nil {
		return false, err
	}

	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", targetRef))
	targetEntry, _, err := rsl.GetLatestUnskippedReferenceEntryForRef(repo, targetRef)
	if err != nil {
		return false, err
	}

	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", featureRef))
	featureEntry, _, err := rsl.GetLatestUnskippedReferenceEntryForRef(repo, featureRef)
	if err != nil {
		return false, err
	}

	mergeTreeID, err := repo.GetMergeTree(targetEntry.TargetID, featureEntry.TargetID)
	if err != nil {
		return false, err
	}

	authorization, githubApproval, err := getAttestationsForIndex(repo, attestationsState, targetRef, targetEntry.TargetID, mergeTreeID)
	if err != nil {
		return false, err
	}

	var (
		rslEntrySignatureNeededForThreshold = false

		gitNamespaceVerified  = false
		pathNamespaceVerified = true // Assume paths are verified until we find out otherwise
	)

	verifiers, err := policyState.FindVerifiersForPath(fmt.Sprintf("%s:%s", gitReferenceRuleScheme, targetRef))
	if err != nil {
		return false, err
	}
	if len(verifiers) == 0 {
		gitNamespaceVerified = true
	}

	for _, verifier := range verifiers {
		// Try verifying twice
		// Once with threshold as it is
		// Once with threshold-- (to account for signature on gitObj)
		err := verifier.Verify(ctx, gitinterface.ZeroHash, authorization, githubApproval)
		if err == nil {
			// we've reached threshold with just the approvals
			gitNamespaceVerified = true
			break
		} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
			// Unexpected error
			return false, err
		}

		// If threshold == 1 and we have approval without gitObj signature, we'll have exited above
		// Only try threshold-- if threshold > 1
		if verifier.Threshold() > 1 {
			// Make a copy before mutating verifier
			verifier := verifier
			verifier.threshold--

			err := verifier.Verify(ctx, gitinterface.ZeroHash, authorization, githubApproval)
			if err == nil {
				rslEntrySignatureNeededForThreshold = true
				gitNamespaceVerified = true
				break
			} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
				return false, err
			}
		}
	}

	if !gitNamespaceVerified {
		return false, fmt.Errorf("not enough approvals to meet Git namespace policies, %w", ErrUnauthorizedSignature)
	}

	hasFileRule, err := policyState.hasFileRule()
	if err != nil {
		return false, err
	}
	if !hasFileRule {
		return rslEntrySignatureNeededForThreshold, nil
	}

	// Verify modified files
	commitIDs, err := repo.GetCommitsBetweenRange(featureEntry.TargetID, targetEntry.TargetID)
	if err != nil {
		return false, err
	}

	commitsVerified := make([]bool, len(commitIDs))
	for i, commitID := range commitIDs {
		// Assume the commit's paths are verified, if a path is left unverified,
		// we flip this later.
		commitsVerified[i] = true

		paths, err := repo.GetFilePathsChangedByCommit(commitID)
		if err != nil {
			return false, err
		}

		pathsVerified := make([]bool, len(paths))
		verifiedUsing := ""
		for j, path := range paths {
			verifiers, err := policyState.FindVerifiersForPath(fmt.Sprintf("%s:%s", fileRuleScheme, path))
			if err != nil {
				return false, err
			}

			if len(verifiers) == 0 {
				pathsVerified[j] = true
				continue
			}

			if len(verifiedUsing) > 0 {
				// We've already verified and identified commit signature, we
				// can just check if that verifier is trusted for the new path.
				// If not found, we don't make any assumptions about it being a
				// failure in case of name mismatches. So, the signature check
				// proceeds as usual.
				for _, verifier := range verifiers {
					if verifier.Name() == verifiedUsing {
						pathsVerified[j] = true
						break
					}
				}
			}

			if pathsVerified[j] {
				continue
			}

			for _, verifier := range verifiers {
				err := verifier.Verify(ctx, commitID, authorization, githubApproval)
				if err == nil {
					// Signature verification succeeded
					pathsVerified[j] = true
					verifiedUsing = verifier.Name()
					break
				} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
					// Unexpected error
					return false, err
				}
			}
		}

		for _, p := range pathsVerified {
			if !p {
				// Flip earlier assumption that commit paths are verified as we
				// find that at least one path wasn't verified successfully
				commitsVerified[i] = false
				break
			}
		}
	}

	for _, c := range commitsVerified {
		if !c {
			pathNamespaceVerified = false
			break
		}
	}

	if !pathNamespaceVerified {
		return false, fmt.Errorf("not enough approvals to meet file namespace policies, %w", ErrUnauthorizedSignature)
	}

	return rslEntrySignatureNeededForThreshold, nil
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

	return rootVerifier.Verify(ctx, gitinterface.ZeroHash, newPolicy.RootEnvelope, nil)
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

	var (
		gitNamespaceVerified  = false
		pathNamespaceVerified = true // Assume paths are verified until we find out otherwise
	)

	// Find authorized verifiers for entry's ref
	verifiers, err := policy.FindVerifiersForPath(fmt.Sprintf("%s:%s", gitReferenceRuleScheme, entry.RefName))
	if err != nil {
		return err
	}

	// No verifiers => no restrictions for the git namespace
	if len(verifiers) == 0 {
		gitNamespaceVerified = true
	}

	authorizationAttestation, githubApprovalAttestation, err := getAttestations(repo, attestationsState, entry)
	if err != nil {
		return err
	}

	// Use each verifier to verify signature
	for _, verifier := range verifiers {
		err := verifier.Verify(ctx, entry.ID, authorizationAttestation, githubApprovalAttestation)
		if err == nil {
			// Signature verification succeeded
			gitNamespaceVerified = true
			break
		} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
			// Unexpected error
			return err
		}
		// Haven't found a valid verifier, continue with next verifier
	}

	if !gitNamespaceVerified {
		return fmt.Errorf("verifying Git namespace policies failed, %w", ErrUnauthorizedSignature)
	}

	hasFileRule, err := policy.hasFileRule()
	if err != nil {
		return err
	}

	if !hasFileRule {
		return nil
	}

	// Verify modified files

	// First, get all commits between the current and last entry for the ref.
	commitIDs, err := getCommits(repo, entry) // note: this is ordered by commit ID
	if err != nil {
		return err
	}

	commitsVerified := make([]bool, len(commitIDs))
	for i, commitID := range commitIDs {
		// Assume the commit's paths are verified, if a path is left unverified,
		// we flip this later.
		commitsVerified[i] = true

		paths, err := repo.GetFilePathsChangedByCommit(commitID)
		if err != nil {
			return err
		}

		pathsVerified := make([]bool, len(paths))
		verifiedUsing := "" // this will be set after one successful verification of the commit to avoid repeated signature verification
		for j, path := range paths {
			verifiers, err := policy.FindVerifiersForPath(fmt.Sprintf("%s:%s", fileRuleScheme, path))
			if err != nil {
				return err
			}

			if len(verifiers) == 0 {
				pathsVerified[j] = true
				continue
			}

			if len(verifiedUsing) > 0 {
				// We've already verified and identified commit signature, we
				// can just check if that verifier is trusted for the new path.
				// If not found, we don't make any assumptions about it being a
				// failure in case of name mismatches. So, the signature check
				// proceeds as usual.
				for _, verifier := range verifiers {
					if verifier.Name() == verifiedUsing {
						pathsVerified[j] = true
						break
					}
				}
			}

			if pathsVerified[j] {
				continue
			}

			for _, verifier := range verifiers {
				err := verifier.Verify(ctx, commitID, authorizationAttestation, githubApprovalAttestation)
				if err == nil {
					// Signature verification succeeded
					pathsVerified[j] = true
					verifiedUsing = verifier.Name()
					break
				} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
					// Unexpected error
					return err
				}
			}
		}

		for _, p := range pathsVerified {
			if !p {
				// Flip earlier assumption that commit paths are verified as we
				// find that at least one path wasn't verified successfully
				commitsVerified[i] = false
				break
			}
		}
	}

	for _, c := range commitsVerified {
		if !c {
			// Set path namespace verified to false as at least one commit's
			// paths weren't verified successfully
			pathNamespaceVerified = false
			break
		}
	}

	if !pathNamespaceVerified {
		return fmt.Errorf("verifying file namespace policies failed, %w", ErrUnauthorizedSignature)
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

	// Find authorized public keys for tag's RSL entry
	verifiers, err := policy.FindVerifiersForPath(fmt.Sprintf("%s:%s", gitReferenceRuleScheme, entry.RefName))
	if err != nil {
		return err
	}

	if len(verifiers) == 0 {
		return nil
	}

	authorizationAttestation, githubApprovalAttestation, err := getAttestations(repo, attestationsState, entry)
	if err != nil {
		return err
	}

	// Use each verifier to verify signature
	rslEntryVerified := false
	for _, verifier := range verifiers {
		err := verifier.Verify(ctx, entry.ID, authorizationAttestation, githubApprovalAttestation)
		if err == nil {
			// Signature verification succeeded
			rslEntryVerified = true
			break
		} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
			// Unexpected error
			return err
		}
		// Haven't found a valid verifier, continue with next verifier
	}

	if !rslEntryVerified {
		return fmt.Errorf("verifying RSL entry failed, %w", ErrUnauthorizedSignature)
	}

	// Verify tag object
	tagObjVerified := false
	for _, verifier := range verifiers {
		// explicitly not looking at the attestation
		// that applies to the _push_
		// thus, we also set threshold to 1
		verifier.threshold = 1

		err := verifier.Verify(ctx, entry.TargetID, nil, nil)
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

func getAttestations(repo *gitinterface.Repository, attestationsState *attestations.Attestations, entry *rsl.ReferenceEntry) (*sslibdsse.Envelope, *sslibdsse.Envelope, error) {
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

	return getAttestationsForIndex(repo, attestationsState, entry.RefName, fromID, entryTreeID)
}

func getAttestationsForIndex(repo *gitinterface.Repository, attestationsState *attestations.Attestations, refName string, fromID, toID gitinterface.Hash) (*sslibdsse.Envelope, *sslibdsse.Envelope, error) {
	if attestationsState == nil {
		return nil, nil, nil
	}

	slog.Debug(fmt.Sprintf("Loading reference authorization for '%s' from '%s' to '%s'...", refName, fromID.String(), toID.String()))
	authorizationAttestation, err := attestationsState.GetReferenceAuthorizationFor(repo, refName, fromID.String(), toID.String())
	if err != nil {
		if !errors.Is(err, attestations.ErrAuthorizationNotFound) {
			return nil, nil, err
		}
	}

	slog.Debug(fmt.Sprintf("Loading GitHub pull request approval attestation for '%s' from '%s' to '%s'...", refName, fromID.String(), toID.String()))
	githubApprovalAttestation, err := attestationsState.GetGitHubPullRequestApprovalAttestationFor(repo, refName, fromID.String(), toID.String())
	if err != nil {
		if !errors.Is(err, attestations.ErrGitHubPullRequestApprovalAttestationNotFound) {
			return nil, nil, err
		}
	}

	return authorizationAttestation, githubApprovalAttestation, nil
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

type Verifier struct {
	repository               *gitinterface.Repository
	name                     string
	keys                     []*tuf.Key
	githubAppApprovalTrusted bool
	githubAppKey             *tuf.Key
	threshold                int
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

// Verify is used to check for a threshold of signatures using the verifier. The
// threshold of signatures may be met using a combination of at most one Git
// signature and signatures embedded in a DSSE envelope. Verify does not inspect
// the envelope's payload, but instead only verifies the signatures. The caller
// must ensure the validity of the envelope's contents.
func (v *Verifier) Verify(ctx context.Context, gitObjectID gitinterface.Hash, authorizationAttestationEnv, githubApprovalAttestationEnv *sslibdsse.Envelope) error {
	if v.threshold < 1 || len(v.keys) < 1 {
		return ErrInvalidVerifier
	}

	allTrustedKeyIDs := set.NewSet[string]()
	for _, key := range v.keys {
		allTrustedKeyIDs.Add(key.KeyID)
	}

	usedKeyIDs := set.NewSet[string]()

	gitObjectVerified := false

	// First, verify the gitObject's signature if one is presented
	if !gitObjectID.IsZero() {
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
				return err
			}
		}
	}

	// If threshold is 1 and the Git signature is verified, we can return
	if v.threshold == 1 && gitObjectVerified {
		return nil
	}

	// TODO: this isn't always an authorization attestation, so the github
	// approval is a weird case
	// Maybe this shouldn't be aware of the special predicate?

	if authorizationAttestationEnv != nil {
		// Second, verify signatures on the attestation, subtracting the threshold
		// by 1 to account for a verified Git signature
		envelopeThreshold := v.threshold
		if gitObjectVerified {
			envelopeThreshold--
		}
		verifiers := make([]sslibdsse.Verifier, 0, len(v.keys))
		for _, key := range v.keys {
			if usedKeyIDs.Has(key.KeyID) {
				// Do not create a DSSE verifier for the key used to verify the Git
				// signature
				continue
			}

			verifier, err := signerverifier.NewSignerVerifierFromTUFKey(key) //nolint:staticcheck
			if err != nil && !errors.Is(err, common.ErrUnknownKeyType) {
				return err
			}
			verifiers = append(verifiers, verifier)
		}

		acceptedKeys, err := dsse.VerifyEnvelope(ctx, authorizationAttestationEnv, verifiers, envelopeThreshold)
		if err != nil {
			if !strings.Contains(err.Error(), "accepted signatures do not match threshold") {
				// we may yet meet threshold with the approval attestation
				return fmt.Errorf("unexpected error: %w", err)
			}
		}
		for _, ak := range acceptedKeys {
			usedKeyIDs.Add(ak.KeyID)
		}
		if usedKeyIDs.Len() >= v.Threshold() {
			// We don't need to look at GitHub approvals
			return nil
		}
	}

	if !v.githubAppApprovalTrusted || githubApprovalAttestationEnv == nil {
		return ErrVerifierConditionsUnmet
	}

	// Verify approval attestation is signed by right key
	verifier, err := signerverifier.NewSignerVerifierFromTUFKey(v.githubAppKey) //nolint:staticcheck
	if err != nil {
		return err
	}
	_, err = dsse.VerifyEnvelope(ctx, githubApprovalAttestationEnv, []sslibdsse.Verifier{verifier}, 1)
	if err != nil {
		return ErrVerifierConditionsUnmet
	}

	payloadBytes, err := githubApprovalAttestationEnv.DecodeB64Payload()
	if err != nil {
		return err
	}

	type tmpStatement struct {
		Type          string                                             `json:"_type"`
		Subject       []*ita.ResourceDescriptor                          `json:"subject"`
		PredicateType string                                             `json:"predicateType"`
		Predicate     *attestations.GitHubPullRequestApprovalAttestation `json:"predicate"`
	}
	stmt := &tmpStatement{Subject: []*ita.ResourceDescriptor{}, Predicate: &attestations.GitHubPullRequestApprovalAttestation{Approvers: []*tuf.Key{}}}
	if err := json.Unmarshal(payloadBytes, stmt); err != nil {
		return err
	}

	for _, key := range stmt.Predicate.Approvers {
		if allTrustedKeyIDs.Has(key.KeyID) {
			usedKeyIDs.Add(key.KeyID)
		}
	}

	if usedKeyIDs.Len() < v.Threshold() {
		return ErrVerifierConditionsUnmet
	}

	return nil
}
