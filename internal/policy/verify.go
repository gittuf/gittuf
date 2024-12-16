// Copyright The gittuf Authors
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
	"github.com/gittuf/gittuf/internal/attestations/authorizations"
	"github.com/gittuf/gittuf/internal/attestations/github"
	githubv01 "github.com/gittuf/gittuf/internal/attestations/github/v01"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	sigstoreverifieropts "github.com/gittuf/gittuf/internal/signerverifier/sigstore/options/verifier"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	ita "github.com/in-toto/attestation/go/v1"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

var (
	ErrUnauthorizedSignature          = errors.New("unauthorized signature")
	ErrInvalidEntryNotSkipped         = errors.New("invalid entry found not marked as skipped")
	ErrLastGoodEntryIsSkipped         = errors.New("entry expected to be unskipped is marked as skipped")
	ErrNoVerifiers                    = errors.New("no verifiers present for verification")
	ErrInvalidVerifier                = errors.New("verifier has invalid parameters (is threshold 0?)")
	ErrVerifierConditionsUnmet        = errors.New("verifier's key and threshold constraints not met")
	ErrCannotVerifyMergeableForTagRef = errors.New("cannot verify mergeable into tag reference")
)

// VerifyRef verifies the signature on the latest RSL entry for the target ref
// using the latest policy. The expected Git ID for the ref in the latest RSL
// entry is returned if the policy verification is successful.
func VerifyRef(ctx context.Context, repo *gitinterface.Repository, target string) (gitinterface.Hash, error) {
	// Find latest entry for target
	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", target))
	latestEntry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(target))
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	return latestEntry.TargetID, VerifyRelativeForRef(ctx, repo, latestEntry, latestEntry, target)
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
	latestEntry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(target))
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	slog.Debug("Verifying all entries...")
	return latestEntry.TargetID, VerifyRelativeForRef(ctx, repo, firstEntry, latestEntry, target)
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

	fromEntry, isRefEntry := fromEntryT.(*rsl.ReferenceEntry)
	if !isRefEntry {
		// TODO: we should instead find the latest reference entry
		// before the entryID and use that
		return gitinterface.ZeroHash, fmt.Errorf("starting entry is not an RSL reference entry")
	}

	// Find latest entry for target
	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", target))
	latestEntry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(target))
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	// Do a relative verify from start entry to the latest entry
	slog.Debug("Verifying all entries...")
	return latestEntry.TargetID, VerifyRelativeForRef(ctx, repo, fromEntry, latestEntry, target)
}

// VerifyMergeable checks if the targetRef can be updated to reflect the changes
// in featureRef. It checks if sufficient authorizations / approvals exist for
// the merge to happen, indicated by the error being nil. Additionally, a
// boolean value is also returned that indicates whether a final authorized
// signature is still necessary via the RSL entry for the merge.
//
// Summary of return combinations:
// (false, err) -> merge is not possible
// (false, nil) -> merge is possible and can be performed by anyone
// (true,  nil) -> merge is possible but it MUST be performed by an authorized
// person for the rule, i.e., an authorized person must sign the merge's RSL
// entry
func VerifyMergeable(ctx context.Context, repo *gitinterface.Repository, targetRef, featureRef string) (bool, error) {
	if strings.HasPrefix(targetRef, gitinterface.TagRefPrefix) {
		return false, ErrCannotVerifyMergeableForTagRef
	}

	var (
		currentPolicy       *State
		currentAttestations *attestations.Attestations
		err                 error
	)

	searcher := newSearcher(repo)

	// Load latest policy
	slog.Debug("Loading latest policy...")
	initialPolicyEntry, err := searcher.FindLatestPolicyEntry()
	if err != nil {
		return false, err
	}
	state, err := LoadState(ctx, repo, initialPolicyEntry)
	if err != nil {
		return false, err
	}
	currentPolicy = state

	// Load latest attestations
	slog.Debug("Loading latest attestations...")
	initialAttestationsEntry, err := searcher.FindLatestAttestationsEntry()
	if err == nil {
		attestationsState, err := attestations.LoadAttestationsForEntry(repo, initialAttestationsEntry)
		if err != nil {
			return false, err
		}
		currentAttestations = attestationsState
	} else if !errors.Is(err, attestations.ErrAttestationsNotFound) {
		// Attestations are not compulsory, so return err only
		// if it's some other error
		return false, err
	}

	var fromID gitinterface.Hash
	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", targetRef))
	targetEntry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(targetRef), rsl.IsUnskipped())
	switch {
	case err == nil:
		fromID = targetEntry.TargetID
	case errors.Is(err, rsl.ErrRSLEntryNotFound):
		fromID = gitinterface.ZeroHash
	default:
		return false, err
	}

	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", featureRef))
	featureEntry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(featureRef), rsl.IsUnskipped())
	if err != nil {
		return false, err
	}

	// We're specifically focused on commit merges here, this doesn't apply to
	// tags
	mergeTreeID, err := repo.GetMergeTree(fromID, featureEntry.TargetID)
	if err != nil {
		return false, err
	}

	authorizationAttestation, approverIDs, err := getApproverAttestationAndKeyIDsForIndex(ctx, repo, currentPolicy, currentAttestations, targetRef, fromID, mergeTreeID, false)
	if err != nil {
		return false, err
	}

	verifiers, err := currentPolicy.FindVerifiersForPath(fmt.Sprintf("%s:%s", gitReferenceRuleScheme, targetRef))
	if err != nil {
		return false, err
	}
	if len(verifiers) == 0 {
		// No verifiers -> not protected
		return false, nil
	}

	var appName string
	if currentPolicy.githubAppApprovalsTrusted {
		appName = currentPolicy.githubAppKeys[0].ID()
	}

	_, rslEntrySignatureNeededForThreshold, err := verifyGitObjectAndAttestationsUsingVerifiers(ctx, verifiers, gitinterface.ZeroHash, authorizationAttestation, appName, approverIDs, true)
	if err != nil {
		return false, fmt.Errorf("not enough approvals to meet Git namespace policies, %w", ErrUnauthorizedSignature)
	}

	if !currentPolicy.hasFileRule {
		return rslEntrySignatureNeededForThreshold, nil
	}

	// Verify modified files
	commitIDs, err := repo.GetCommitsBetweenRange(featureEntry.TargetID, fromID)
	if err != nil {
		return false, err
	}

	for _, commitID := range commitIDs {
		paths, err := repo.GetFilePathsChangedByCommit(commitID)
		if err != nil {
			return false, err
		}

		verifiedUsing := "" // this will be set after one successful verification of the commit to avoid repeated signature verification
		for _, path := range paths {
			verifiers, err := currentPolicy.FindVerifiersForPath(fmt.Sprintf("%s:%s", fileRuleScheme, path))
			if err != nil {
				return false, err
			}

			verified := false
			if len(verifiedUsing) > 0 {
				// We've already verified and identified the commit signature,
				// we can just check if that verifier is trusted for the new
				// path.  If not found, we don't make any assumptions about it
				// being a failure in case of name mismatches. So, the signature
				// check proceeds as usual.
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

			// We don't use verifyMergeable=true here
			// File verification rules are not met using the signature on the
			// RSL entry, so we don't count threshold-1 here
			verifiedUsing, _, err = verifyGitObjectAndAttestationsUsingVerifiers(ctx, verifiers, commitID, authorizationAttestation, appName, approverIDs, false)
			if err != nil {
				return false, fmt.Errorf("verifying file namespace policies failed, %w", ErrUnauthorizedSignature)
			}
		}
	}

	return rslEntrySignatureNeededForThreshold, nil
}

// VerifyRelativeForRef verifies the RSL between specified start and end entries
// using the provided policy entry for the first entry.
func VerifyRelativeForRef(ctx context.Context, repo *gitinterface.Repository, firstEntry, lastEntry *rsl.ReferenceEntry, target string) error {
	/*
		require firstEntry != nil
		require lastEntry != nil
		require target != ""
	*/

	var (
		currentPolicy       *State
		currentAttestations *attestations.Attestations
		err                 error
	)

	searcher := newSearcher(repo)

	// Load policy applicable at firstEntry
	slog.Debug(fmt.Sprintf("Loading policy applicable at first entry '%s'...", firstEntry.ID.String()))
	initialPolicyEntry, err := searcher.FindPolicyEntryFor(firstEntry)
	if err == nil {
		state, err := LoadState(ctx, repo, initialPolicyEntry)
		if err != nil {
			return err
		}
		currentPolicy = state
	} else if !errors.Is(err, ErrPolicyNotFound) {
		// Searcher gives us nil when firstEntry is the very first entry
		// or close to it (i.e., before a policy was applied)
		return err
	}
	// require currentPolicy != nil || parent(firstEntry) == nil

	slog.Debug(fmt.Sprintf("Loading attestations applicable at first entry '%s'...", firstEntry.ID.String()))
	initialAttestationsEntry, err := searcher.FindAttestationsEntryFor(firstEntry)
	if err == nil {
		attestationsState, err := attestations.LoadAttestationsForEntry(repo, initialAttestationsEntry)
		if err != nil {
			return err
		}
		currentAttestations = attestationsState
	} else if !errors.Is(err, attestations.ErrAttestationsNotFound) {
		// Attestations are not compulsory, so return err only
		// if it's some other error
		return err
	}
	// require currentAttestations != nil || (entry.Ref != attestations.Ref for entry in 0..firstEntry)

	// Enumerate RSL entries between firstEntry and lastEntry, ignoring irrelevant ones
	slog.Debug("Identifying all entries in range...")
	entries, annotations, err := rsl.GetReferenceEntriesInRangeForRef(repo, firstEntry.ID, lastEntry.ID, target)
	if err != nil {
		return err
	}
	// require len(entries) != 0

	// Verify each entry, looking for a fix when an invalid entry is encountered
	var invalidEntry *rsl.ReferenceEntry
	var verificationErr error
	for len(entries) != 0 {
		// invariant invalidEntry == nil || inRecoveryMode() == true
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
				if entry.ID.Equal(firstEntry.ID) {
					// We've already loaded this policy
					continue
				}

				newPolicy, err := loadStateForEntry(repo, entry)
				if err != nil {
					return err
				}
				// require newPolicy != nil

				if currentPolicy != nil {
					// currentPolicy can be nil when
					// verifying from the beginning of the
					// RSL entry and we only have staging
					// refs
					slog.Debug("Verifying new policy using current policy...")
					if err := currentPolicy.VerifyNewState(ctx, newPolicy); err != nil {
						return err
					}
					slog.Debug("Updating current policy...")
				} else {
					slog.Debug("Setting current policy...")
				}

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
			if currentPolicy == nil {
				return ErrPolicyNotFound
			}
			if err := verifyEntry(ctx, repo, currentPolicy, currentAttestations, entry); err != nil {
				slog.Debug(fmt.Sprintf("Violation found: %s", err.Error()))
				slog.Debug("Checking if entry has been revoked...")
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
		lastGoodEntry, lastGoodEntryAnnotations, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(invalidEntry.RefName), rsl.BeforeEntryID(invalidEntry.ID), rsl.IsUnskipped())
		if err != nil {
			return err
		}
		slog.Debug("Verifying identified last valid entry has not been revoked...")
		if lastGoodEntry.SkippedBy(lastGoodEntryAnnotations) {
			return ErrLastGoodEntryIsSkipped
		}
		// require lastGoodEntry != nil

		// TODO: what if the very first entry for a ref is a violation?

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
		slog.Debug("Entry is for a Git tag, using tag verification workflow...")
		return verifyTagEntry(ctx, repo, policy, attestationsState, entry)
	}

	// Load the applicable reference authorization and approvals from trusted
	// code review systems
	authorizationAttestation, approverKeyIDs, err := getApproverAttestationAndKeyIDs(ctx, repo, policy, attestationsState, entry)
	if err != nil {
		return err
	}

	// Verify Git namespace policies using the RSL entry and attestations
	if _, _, err := verifyGitObjectAndAttestations(ctx, policy, fmt.Sprintf("%s:%s", gitReferenceRuleScheme, entry.RefName), entry.ID, authorizationAttestation, approverKeyIDs); err != nil {
		return fmt.Errorf("verifying Git namespace policies failed, %w", ErrUnauthorizedSignature)
	}

	// Check if policy has file rules at all for efficiency
	if !policy.hasFileRule {
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
			if len(verifiers) == 0 {
				// Path is not protected
				verified = true
			} else if len(verifiedUsing) > 0 {
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

			// TODO: app name
			appName := ""
			if policy.githubAppApprovalsTrusted {
				appName = policy.githubAppKeys[0].ID()
			}
			verifiedUsing, _, err = verifyGitObjectAndAttestationsUsingVerifiers(ctx, verifiers, commitID, authorizationAttestation, appName, approverKeyIDs, false)
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
	// TODO: app name
	appName := ""
	if policy.githubAppApprovalsTrusted {
		appName = policy.githubAppKeys[0].ID()
	}
	if _, _, err := verifyGitObjectAndAttestationsUsingVerifiers(ctx, verifiers, entry.ID, authorizationAttestation, appName, approverKeyIDs, false); err != nil {
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
	priorRefEntry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(entry.RefName), rsl.BeforeEntryID(entry.ID))
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

	// We need to handle the case where we're approving a tag
	// For a tag, the expected toID in the approval is the commit the tag points to
	// Otherwise, the expected toID is the tree the commit points to
	var (
		toID  gitinterface.Hash
		isTag bool
	)
	if strings.HasPrefix(entry.RefName, gitinterface.TagRefPrefix) {
		isTag = true

		toID, err = repo.GetTagTarget(entry.TargetID)
	} else {
		toID, err = repo.GetCommitTreeID(entry.TargetID)
	}
	if err != nil {
		return nil, nil, err
	}

	return getApproverAttestationAndKeyIDsForIndex(ctx, repo, policy, attestationsState, entry.RefName, fromID, toID, isTag)
}

func getApproverAttestationAndKeyIDsForIndex(ctx context.Context, repo *gitinterface.Repository, policy *State, attestationsState *attestations.Attestations, targetRef string, fromID, toID gitinterface.Hash, isTag bool) (*sslibdsse.Envelope, *set.Set[string], error) {
	if attestationsState == nil {
		return nil, nil, nil
	}

	slog.Debug(fmt.Sprintf("Finding reference authorization attestations for '%s' from '%s' to '%s'...", targetRef, fromID.String(), toID.String()))
	authorizationAttestation, err := attestationsState.GetReferenceAuthorizationFor(repo, targetRef, fromID.String(), toID.String())
	if err != nil {
		if !errors.Is(err, authorizations.ErrAuthorizationNotFound) {
			return nil, nil, err
		}
	}

	approverIdentities := set.NewSet[string]()

	// When we add other code review systems, we can move this into a
	// generalized helper that inspects the attestations for each system trusted
	// in policy.
	// We only use this flow right now for non-tags as tags cannot be approved
	// on currently supported systems
	// TODO: support multiple apps / threshold per system
	if !isTag && policy.githubAppApprovalsTrusted {
		slog.Debug("GitHub pull request approvals are trusted, loading applicable attestations...")

		appName := policy.githubAppKeys[0].ID()

		githubApprovalAttestation, err := attestationsState.GetGitHubPullRequestApprovalAttestationFor(repo, appName, targetRef, fromID.String(), toID.String())
		if err != nil {
			if !errors.Is(err, github.ErrPullRequestApprovalAttestationNotFound) {
				return nil, nil, err
			}
		}

		// if it exists
		if githubApprovalAttestation != nil {
			slog.Debug("GitHub pull request approval found, verifying attestation signature...")
			approvalVerifier := &Verifier{
				repository: policy.repository,
				name:       tuf.GitHubAppRoleName,
				principals: policy.githubAppKeys,
				threshold:  1, // TODO: support higher threshold
			}
			_, err := approvalVerifier.Verify(ctx, nil, githubApprovalAttestation)
			if err != nil {
				return nil, nil, fmt.Errorf("failed to verify GitHub app approval attestation, signed by untrusted key")
			}

			payloadBytes, err := githubApprovalAttestation.DecodeB64Payload()
			if err != nil {
				return nil, nil, err
			}

			// TODO: support multiple versions
			type tmpStatement struct {
				Type          string                                    `json:"_type"`
				Subject       []*ita.ResourceDescriptor                 `json:"subject"`
				PredicateType string                                    `json:"predicateType"`
				Predicate     *githubv01.PullRequestApprovalAttestation `json:"predicate"`
			}
			stmt := new(tmpStatement)
			if err := json.Unmarshal(payloadBytes, stmt); err != nil {
				return nil, nil, err
			}

			for _, approver := range stmt.Predicate.GetApprovers() {
				approverIdentities.Add(approver)
			}
		}
	}

	return authorizationAttestation, approverIdentities, nil
}

// getCommits identifies the commits introduced to the entry's ref since the
// last RSL entry for the same ref. These commits are then verified for file
// policies.
func getCommits(repo *gitinterface.Repository, entry *rsl.ReferenceEntry) ([]gitinterface.Hash, error) {
	firstEntry := false

	priorRefEntry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference(entry.RefName), rsl.BeforeEntryID(entry.ID))
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

func verifyGitObjectAndAttestations(ctx context.Context, policy *State, target string, gitID gitinterface.Hash, authorizationAttestation *sslibdsse.Envelope, approverKeyIDs *set.Set[string]) (string, bool, error) {
	verifiers, err := policy.FindVerifiersForPath(target)
	if err != nil {
		return "", false, err
	}

	if len(verifiers) == 0 {
		// This target is not protected by gittuf policy
		return "", false, nil
	}

	// TODO: app name
	appName := ""
	if policy.githubAppApprovalsTrusted {
		appName = policy.githubAppKeys[0].ID()
	}
	return verifyGitObjectAndAttestationsUsingVerifiers(ctx, verifiers, gitID, authorizationAttestation, appName, approverKeyIDs, false)
}

func verifyGitObjectAndAttestationsUsingVerifiers(ctx context.Context, verifiers []*Verifier, gitID gitinterface.Hash, authorizationAttestation *sslibdsse.Envelope, appName string, approverIDs *set.Set[string], verifyMergeable bool) (string, bool, error) {
	if len(verifiers) == 0 {
		return "", false, ErrNoVerifiers
	}

	var (
		verifiedUsing                       string
		rslEntrySignatureNeededForThreshold bool
	)
	for _, verifier := range verifiers {
		trustedPrincipalIDs := verifier.TrustedPrincipalIDs()

		usedPrincipalIDs, err := verifier.Verify(ctx, gitID, authorizationAttestation)
		if err == nil {
			// We meet requirements just from the authorization attestation's sigs
			verifiedUsing = verifier.Name()
			break
		} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
			return "", false, err
		}

		if approverIDs != nil {
			slog.Debug("Using approvers from code review tool attestations...")
			// Unify the principalIDs we've already used with that listed in
			// approval attestation
			// We ensure that someone who has signed an attestation and is listed in
			// the approval attestation is only counted once
			for _, approverID := range approverIDs.Contents() {
				// For each approver ID from the app attestation, we try to see
				// if it matches a principal in the current verifiers.
				for _, principal := range verifier.principals {
					slog.Debug(fmt.Sprintf("Checking if approver identity '%s' matches '%s'...", approverID, principal.ID()))
					if usedPrincipalIDs.Has(principal.ID()) {
						// This principal has already been counted towards the
						// threshold
						slog.Debug(fmt.Sprintf("Principal '%s' has already been counted towards threshold, skipping...", principal.ID()))
						continue
					}

					// We can only match against a principal if it has a notion
					// of associated identities
					// Right now, this is just tufv02.Person
					if principal, isV02 := principal.(*tufv02.Person); isV02 {
						if associatedIdentity, has := principal.AssociatedIdentities[appName]; has && associatedIdentity == approverID {
							// The approver ID from the issuer (appName) matches
							// the principal's associated identity for the same
							// issuer!
							slog.Debug(fmt.Sprintf("Principal '%s' has associated identity '%s', counting principal towards threshold...", principal.ID(), approverID))
							usedPrincipalIDs.Add(principal.ID())
							break
						}
					}
				}
			}
		}

		// Get a list of used principals that are also trusted by the verifier
		trustedUsedPrincipalIDs := trustedPrincipalIDs.Intersection(usedPrincipalIDs)
		if trustedUsedPrincipalIDs.Len() >= verifier.Threshold() {
			// With approvals, we now meet threshold!
			slog.Debug(fmt.Sprintf("Counted '%d' principals towards threshold '%d' for '%s', threshold met!", trustedUsedPrincipalIDs.Len(), verifier.Threshold(), verifier.Name()))
			verifiedUsing = verifier.Name()
			break
		}

		// If verifyMergeable is true, we only need to meet threshold - 1
		if verifyMergeable && verifier.Threshold() > 1 {
			if trustedUsedPrincipalIDs.Len() >= verifier.Threshold()-1 {
				slog.Debug(fmt.Sprintf("Counted '%d' principals towards threshold '%d' for '%s', policies can be met if the merge is by authorized person!", trustedUsedPrincipalIDs.Len(), verifier.Threshold(), verifier.Name()))
				verifiedUsing = verifier.Name()
				rslEntrySignatureNeededForThreshold = true
				break
			}
		}
	}

	if verifiedUsing != "" {
		return verifiedUsing, rslEntrySignatureNeededForThreshold, nil
	}

	return "", false, ErrVerifierConditionsUnmet
}

type Verifier struct {
	repository *gitinterface.Repository
	name       string
	principals []tuf.Principal
	threshold  int
}

func (v *Verifier) Name() string {
	return v.name
}

func (v *Verifier) Threshold() int {
	return v.threshold
}

func (v *Verifier) TrustedPrincipalIDs() *set.Set[string] {
	principalIDs := set.NewSet[string]()
	for _, principal := range v.principals {
		principalIDs.Add(principal.ID())
	}

	return principalIDs
}

// Verify is used to check for a threshold of signatures using the verifier. The
// threshold of signatures may be met using a combination of at most one Git
// signature and signatures embedded in a DSSE envelope. Verify does not inspect
// the envelope's payload, but instead only verifies the signatures. The caller
// must ensure the validity of the envelope's contents.
func (v *Verifier) Verify(ctx context.Context, gitObjectID gitinterface.Hash, env *sslibdsse.Envelope) (*set.Set[string], error) {
	if v.threshold < 1 || len(v.principals) < 1 {
		return nil, ErrInvalidVerifier
	}

	// usedPrincipalIDs is ultimately returned to track the set of principals
	// who have been authenticated
	usedPrincipalIDs := set.NewSet[string]()

	// usedKeyIDs is tracked to ensure a key isn't duplicated between two
	// principals, allowing two principals to meet a threshold using the same
	// key
	usedKeyIDs := set.NewSet[string]()

	// gitObjectVerified is set to true if the gitObjectID's signature is
	// verified
	gitObjectVerified := false

	// First, verify the gitObject's signature if one is presented
	if gitObjectID != nil && !gitObjectID.IsZero() {
		slog.Debug(fmt.Sprintf("Verifying signature of Git object with ID '%s'...", gitObjectID.String()))
		for _, principal := range v.principals {
			// there are multiple keys we must try
			keys := principal.Keys()

			for _, key := range keys {
				err := v.repository.VerifySignature(ctx, gitObjectID, key)
				if err == nil {
					// Signature verification succeeded
					slog.Debug(fmt.Sprintf("Public key '%s' belonging to principal '%s' successfully used to verify signature of Git object '%s', counting '%s' towards threshold...", key.KeyID, principal.ID(), gitObjectID.String(), principal.ID()))
					usedPrincipalIDs.Add(principal.ID())
					usedKeyIDs.Add(key.KeyID)
					gitObjectVerified = true

					// No need to try the other keys for this principal, break
					break
				}
				if errors.Is(err, gitinterface.ErrUnknownSigningMethod) {
					// TODO: this should be removed once we have unified signing
					// methods across metadata and git signatures
					continue
				}
				if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
					return nil, err
				}
			}

			if gitObjectVerified {
				// No need to try other principals, break
				break
			}
		}
	}

	// If threshold is 1 and the Git signature is verified, we can return
	if v.threshold == 1 && gitObjectVerified {
		return usedPrincipalIDs, nil
	}

	if env != nil {
		// Second, verify signatures on the envelope

		// We have to verify the envelope independently for each principal
		// trusted in the verifier as a principal may have multiple keys
		// associated with them.
		for _, principal := range v.principals {
			if usedPrincipalIDs.Has(principal.ID()) {
				// Do not verify using this principal as they were verified for
				// the Git signature
				slog.Debug(fmt.Sprintf("Principal '%s' has already been counted towards the threshold, skipping...", principal.ID()))
				continue
			}

			principalVerifiers := []sslibdsse.Verifier{}

			keys := principal.Keys()
			for _, key := range keys {
				if usedKeyIDs.Has(key.KeyID) {
					// this key has been encountered before, possibly because
					// another Principal included this key
					slog.Debug(fmt.Sprintf("Key with ID '%s' has already been used to verify a signature, skipping...", key.KeyID))
					continue
				}

				var (
					dsseVerifier sslibdsse.Verifier
					err          error
				)
				switch key.KeyType {
				case ssh.KeyType:
					slog.Debug(fmt.Sprintf("Found SSH key '%s'...", key.KeyID))
					dsseVerifier, err = ssh.NewVerifierFromKey(key)
					if err != nil {
						return nil, err
					}
				case gpg.KeyType:
					slog.Debug(fmt.Sprintf("Found GPG key '%s', cannot use for DSSE signature verification yet...", key.KeyID))
					continue
				case sigstore.KeyType:
					slog.Debug(fmt.Sprintf("Found Sigstore key '%s'...", key.KeyID))
					opts := []sigstoreverifieropts.Option{}
					config, err := v.repository.GetGitConfig()
					if err != nil {
						return nil, err
					}
					if rekorURL, has := config[sigstore.GitConfigRekor]; has {
						slog.Debug(fmt.Sprintf("Using '%s' as Rekor server...", rekorURL))
						opts = append(opts, sigstoreverifieropts.WithRekorURL(rekorURL))
					}

					dsseVerifier = sigstore.NewVerifierFromIdentityAndIssuer(key.KeyVal.Identity, key.KeyVal.Issuer, opts...)
				case signerverifier.ED25519KeyType:
					// These are only used to verify old policy metadata signed before the ssh-signer was added
					slog.Debug(fmt.Sprintf("Found legacy ED25519 key '%s' in custom securesystemslib format...", key.KeyID))
					dsseVerifier, err = signerverifier.NewED25519SignerVerifierFromSSLibKey(key)
					if err != nil {
						return nil, err
					}
				case signerverifier.RSAKeyType:
					// These are only used to verify old policy metadata signed before the ssh-signer was added
					slog.Debug(fmt.Sprintf("Found legacy RSA key '%s' in custom securesystemslib format...", key.KeyID))
					dsseVerifier, err = signerverifier.NewRSAPSSSignerVerifierFromSSLibKey(key)
					if err != nil {
						return nil, err
					}
				case signerverifier.ECDSAKeyType:
					// These are only used to verify old policy metadata signed before the ssh-signer was added
					slog.Debug(fmt.Sprintf("Found legacy ECDSA key '%s' in custom securesystemslib format...", key.KeyID))
					dsseVerifier, err = signerverifier.NewECDSASignerVerifierFromSSLibKey(key)
					if err != nil {
						return nil, err
					}
				default:
					return nil, common.ErrUnknownKeyType
				}

				principalVerifiers = append(principalVerifiers, dsseVerifier)
			}

			// We have the principal's verifiers: use that to verify the envelope
			if len(principalVerifiers) == 0 {
				// TODO: remove this when we have signing method unification
				// across git and dsse
				continue
			}

			// We set threshold to 1 as we only need one of the keys for this
			// principal to be matched. If more than one key is matched and
			// returned in acceptedKeys, we count this only once towards the
			// principal and therefore the verifier's threshold. However, for
			// safety, we count both keys. If two principals share keys, this
			// can lead to a problem meeting thresholds. Arguably, they
			// shouldn't be sharing keys, so this seems reasonable.
			acceptedKeys, err := dsse.VerifyEnvelope(ctx, env, principalVerifiers, 1)
			if err != nil && !strings.Contains(err.Error(), "accepted signatures do not match threshold") {
				return nil, err
			}

			for _, key := range acceptedKeys {
				// Mark all accepted keys as used: this doesn't count towards
				// the threshold directly, but if another principal has the same
				// key, they may not be counted towards the threshold
				usedKeyIDs.Add(key.KeyID)
			}

			if len(acceptedKeys) > 0 {
				// We've verified this principal, one closer to the threshold
				usedPrincipalIDs.Add(principal.ID())
			}
		}

		if usedPrincipalIDs.Len() >= v.Threshold() {
			return usedPrincipalIDs, nil
		}
	}

	// Return usedPrincipalIDs so the consumer can decide what to do with the
	// principals that were used
	return usedPrincipalIDs, ErrVerifierConditionsUnmet
}
