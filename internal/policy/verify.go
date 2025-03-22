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
	"github.com/gittuf/gittuf/internal/cache"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	ita "github.com/in-toto/attestation/go/v1"
)

var (
	ErrVerificationFailed             = errors.New("gittuf policy verification failed")
	ErrInvalidEntryNotSkipped         = errors.New("invalid entry found not marked as skipped")
	ErrLastGoodEntryIsSkipped         = errors.New("entry expected to be unskipped is marked as skipped")
	ErrNoVerifiers                    = errors.New("no verifiers present for verification")
	ErrInvalidVerifier                = errors.New("verifier has invalid parameters (is threshold 0?)")
	ErrVerifierConditionsUnmet        = errors.New("verifier's key and threshold constraints not met")
	ErrCannotVerifyMergeableForTagRef = errors.New("cannot verify mergeable into tag reference")
)

// PolicyVerifier implements various gittuf verification workflows.
type PolicyVerifier struct { //nolint:revive
	// We want to call this PolicyVerifier to avoid any confusion with
	// SignatureVerifier.

	repo     *gitinterface.Repository
	searcher searcher

	persistentCacheEnabled bool
	persistentCache        *cache.Persistent
}

func NewPolicyVerifier(repo *gitinterface.Repository) *PolicyVerifier {
	searcher := newSearcher(repo)
	verifier := &PolicyVerifier{
		repo:     repo,
		searcher: searcher,
	}

	if searcher, isCacheSearcher := searcher.(*cacheSearcher); isCacheSearcher {
		verifier.persistentCacheEnabled = true
		verifier.persistentCache = searcher.persistentCache
	}

	return verifier
}

// VerifyRef verifies the signature on the latest RSL entry for the target ref
// using the latest policy. The expected Git ID for the ref in the latest RSL
// entry is returned if the policy verification is successful.
func (v *PolicyVerifier) VerifyRef(ctx context.Context, target string) (gitinterface.Hash, error) {
	// Find latest entry for target
	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", target))
	latestEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(v.repo, rsl.ForReference(target))
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	return latestEntry.GetTargetID(), v.VerifyRelativeForRef(ctx, latestEntry, latestEntry, target)
}

// VerifyRefFull verifies the entire RSL for the target ref from the first
// entry. The expected Git ID for the ref in the latest RSL entry is returned if
// the policy verification is successful.
func (v *PolicyVerifier) VerifyRefFull(ctx context.Context, target string) (gitinterface.Hash, error) {
	// Trace RSL back to the start
	slog.Debug(fmt.Sprintf("Identifying first RSL entry for '%s'...", target))
	var (
		firstEntry rsl.ReferenceUpdaterEntry
		err        error
	)
	switch v.persistentCacheEnabled {
	case true:
		slog.Debug("Cache is enabled, checking for last verified entry...")
		entryNumber, entryID := v.persistentCache.GetLastVerifiedEntryForRef(target)
		if entryNumber != 0 {
			firstEntry, err = loadRSLReferenceUpdaterEntry(v.repo, entryID)
			if err != nil {
				return gitinterface.ZeroHash, err
			}

			// break because we've loaded the entry and don't need to fallthrough
			break
		}
		slog.Debug("Cache doesn't have last verified entry for ref...")
		fallthrough
	case false:
		firstEntry, _, err = rsl.GetFirstReferenceUpdaterEntryForRef(v.repo, target)
		if err != nil {
			return gitinterface.ZeroHash, err
		}
	}

	// Find latest entry for target
	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", target))
	latestEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(v.repo, rsl.ForReference(target))
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	slog.Debug("Verifying all entries...")
	return latestEntry.GetTargetID(), v.VerifyRelativeForRef(ctx, firstEntry, latestEntry, target)
}

// VerifyRefFromEntry performs verification for the reference from a specific
// RSL entry. The expected Git ID for the ref in the latest RSL entry is
// returned if the policy verification is successful.
func (v *PolicyVerifier) VerifyRefFromEntry(ctx context.Context, target string, entryID gitinterface.Hash) (gitinterface.Hash, error) {
	// Load starting point entry
	slog.Debug("Identifying starting RSL entry...")
	fromEntryT, err := rsl.GetEntry(v.repo, entryID)
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
	latestEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(v.repo, rsl.ForReference(target))
	if err != nil {
		return gitinterface.ZeroHash, err
	}

	// Do a relative verify from start entry to the latest entry
	slog.Debug("Verifying all entries...")
	return latestEntry.GetTargetID(), v.VerifyRelativeForRef(ctx, fromEntry, latestEntry, target)
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
func (v *PolicyVerifier) VerifyMergeable(ctx context.Context, targetRef, featureRef string) (bool, error) {
	if strings.HasPrefix(targetRef, gitinterface.TagRefPrefix) {
		return false, ErrCannotVerifyMergeableForTagRef
	}

	var fromID gitinterface.Hash
	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", targetRef))
	targetEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(v.repo, rsl.ForReference(targetRef), rsl.IsUnskipped())
	switch {
	case err == nil:
		fromID = targetEntry.GetTargetID()
	case errors.Is(err, rsl.ErrRSLEntryNotFound):
		fromID = gitinterface.ZeroHash
	default:
		return false, err
	}

	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", featureRef))
	featureEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(v.repo, rsl.ForReference(featureRef), rsl.IsUnskipped())
	if err != nil {
		return false, err
	}

	return v.verifyMergeable(ctx, targetRef, fromID, featureEntry.GetTargetID())
}

// VerifyMergeableForCommit checks if the targetRef can be updated to reflect
// the changes in featureID. It checks if sufficient authorizations / approvals
// exist for the merge to happen, indicated by the error being nil.
// Additionally, a boolean value is also returned that indicates whether a final
// authorized signature is still necessary via the RSL entry for the merge.
// Note: this function DOES NOT use the RSL to identify the tip of the feature
// ref.
//
// Summary of return combinations:
// (false, err) -> merge is not possible
// (false, nil) -> merge is possible and can be performed by anyone
// (true,  nil) -> merge is possible but it MUST be performed by an authorized
// person for the rule, i.e., an authorized person must sign the merge's RSL
// entry
func (v *PolicyVerifier) VerifyMergeableForCommit(ctx context.Context, targetRef string, featureID gitinterface.Hash) (bool, error) {
	if strings.HasPrefix(targetRef, gitinterface.TagRefPrefix) {
		return false, ErrCannotVerifyMergeableForTagRef
	}

	var fromID gitinterface.Hash
	slog.Debug(fmt.Sprintf("Identifying latest RSL entry for '%s'...", targetRef))
	targetEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(v.repo, rsl.ForReference(targetRef), rsl.IsUnskipped())
	switch {
	case err == nil:
		fromID = targetEntry.GetTargetID()
	case errors.Is(err, rsl.ErrRSLEntryNotFound):
		fromID = gitinterface.ZeroHash
	default:
		return false, err
	}

	return v.verifyMergeable(ctx, targetRef, fromID, featureID)
}

func (v *PolicyVerifier) verifyMergeable(ctx context.Context, targetRef string, fromID, featureID gitinterface.Hash) (bool, error) {
	// We're specifically focused on commit merges here, this doesn't apply to
	// tags
	mergeTreeID, err := v.repo.GetMergeTree(fromID, featureID)
	if err != nil {
		return false, err
	}
	var (
		currentPolicy       *State
		currentAttestations *attestations.Attestations
	)

	// Load latest policy
	slog.Debug("Loading latest policy...")
	initialPolicyEntry, err := v.searcher.FindLatestPolicyEntry()
	if err != nil {
		return false, err
	}
	state, err := LoadState(ctx, v.repo, initialPolicyEntry)
	if err != nil {
		return false, err
	}
	currentPolicy = state

	// Load latest attestations
	slog.Debug("Loading latest attestations...")
	initialAttestationsEntry, err := v.searcher.FindLatestAttestationsEntry()
	if err == nil {
		attestationsState, err := attestations.LoadAttestationsForEntry(v.repo, initialAttestationsEntry)
		if err != nil {
			return false, err
		}
		currentAttestations = attestationsState
	} else if !errors.Is(err, attestations.ErrAttestationsNotFound) {
		// Attestations are not compulsory, so return err only
		// if it's some other error
		return false, err
	}

	authorizationAttestation, approverIDs, err := getApproverAttestationAndKeyIDsForIndex(ctx, v.repo, currentPolicy, currentAttestations, targetRef, fromID, mergeTreeID, false)
	if err != nil {
		return false, err
	}

	_, rslEntrySignatureNeededForThreshold, err := verifyGitObjectAndAttestations(ctx, currentPolicy, fmt.Sprintf("%s:%s", gitReferenceRuleScheme, targetRef), gitinterface.ZeroHash, authorizationAttestation, withApproverPrincipalIDs(approverIDs), withVerifyMergeable())
	if err != nil {
		return false, fmt.Errorf("not enough approvals to meet Git namespace policies, %w", ErrVerificationFailed)
	}

	if !currentPolicy.hasFileRule {
		return rslEntrySignatureNeededForThreshold, nil
	}

	// Verify modified files
	commitIDs, err := v.repo.GetCommitsBetweenRange(featureID, fromID)
	if err != nil {
		return false, err
	}

	for _, commitID := range commitIDs {
		paths, err := v.repo.GetFilePathsChangedByCommit(commitID)
		if err != nil {
			return false, err
		}

		verifiedUsing := "" // this will be set after one successful verification of the commit to avoid repeated signature verification
		for _, path := range paths {
			// If we've already verified and identified commit signature, we can
			// just check if that verifier is trusted for the new path. If not
			// found, we don't make any assumptions about it being a failure in
			// case of name mismatches. So, the signature check proceeds as
			// usual. Also, we don't use verifyMergeable=true here. File
			// verification rules are not met using the signature on the RSL
			// entry, so we don't count threshold-1 here.
			verifiedUsing, _, err = verifyGitObjectAndAttestations(ctx, currentPolicy, fmt.Sprintf("%s:%s", fileRuleScheme, path), commitID, authorizationAttestation, withApproverPrincipalIDs(approverIDs), withTrustedVerifier(verifiedUsing))
			if err != nil {
				return false, fmt.Errorf("verifying file namespace policies failed, %w", ErrVerificationFailed)
			}
		}
	}

	return rslEntrySignatureNeededForThreshold, nil
}

// VerifyRelativeForRef verifies the RSL between specified start and end entries
// using the provided policy entry for the first entry.
func (v *PolicyVerifier) VerifyRelativeForRef(ctx context.Context, firstEntry, lastEntry rsl.ReferenceUpdaterEntry, target string) error {
	/*
		require firstEntry != nil
		require lastEntry != nil
		require target != ""
	*/

	if v.persistentCacheEnabled {
		defer v.persistentCache.Commit(v.repo) //nolint:errcheck
	}

	var (
		currentPolicy       *State
		currentAttestations *attestations.Attestations
		err                 error
	)

	// Load policy applicable at firstEntry
	slog.Debug(fmt.Sprintf("Loading policy applicable at first entry '%s'...", firstEntry.GetID().String()))
	initialPolicyEntry, err := v.searcher.FindPolicyEntryFor(firstEntry)
	if err == nil {
		state, err := LoadState(ctx, v.repo, initialPolicyEntry)
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

	slog.Debug(fmt.Sprintf("Loading attestations applicable at first entry '%s'...", firstEntry.GetID().String()))
	initialAttestationsEntry, err := v.searcher.FindAttestationsEntryFor(firstEntry)
	if err == nil {
		attestationsState, err := attestations.LoadAttestationsForEntry(v.repo, initialAttestationsEntry)
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
	entries, annotations, err := rsl.GetReferenceUpdaterEntriesInRangeForRef(v.repo, firstEntry.GetID(), lastEntry.GetID(), target)
	if err != nil {
		return err
	}
	// require len(entries) != 0

	// Verify each entry, looking for a fix when an invalid entry is encountered
	var invalidEntry rsl.ReferenceUpdaterEntry
	var verificationErr error
	for len(entries) != 0 {
		// invariant invalidEntry == nil || inRecoveryMode() == true
		if invalidEntry == nil {
			// Pop entry from queue
			entry := entries[0]
			entries = entries[1:]

			slog.Debug(fmt.Sprintf("Verifying entry '%s'...", entry.GetID().String()))

			switch entry := entry.(type) {
			case *rsl.PropagationEntry:
				slog.Debug(fmt.Sprintf("Entry '%s' is propagation entry, proceeding...", entry.GetID().String()))
				continue

			case *rsl.ReferenceEntry:
				slog.Debug("Checking if entry is for policy staging reference...")
				if entry.GetRefName() == PolicyStagingRef {
					continue
				}
				slog.Debug("Checking if entry is for policy reference...")
				if entry.GetRefName() == PolicyRef {
					if entry.GetID().Equal(firstEntry.GetID()) {
						// We've already loaded this policy
						continue
					}

					newPolicy, err := loadStateForEntry(v.repo, entry)
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

					if v.persistentCacheEnabled {
						v.persistentCache.InsertPolicyEntryNumber(entry.GetNumber(), entry.GetID())
					}

					continue
				}

				slog.Debug("Checking if entry is for attestations reference...")
				if entry.GetRefName() == attestations.Ref {
					newAttestationsState, err := attestations.LoadAttestationsForEntry(v.repo, entry)
					if err != nil {
						return err
					}

					currentAttestations = newAttestationsState

					if v.persistentCacheEnabled {
						v.persistentCache.InsertAttestationEntryNumber(entry.GetNumber(), entry.GetID())
					}

					continue
				}

				slog.Debug("Verifying changes...")
				if currentPolicy == nil {
					return ErrPolicyNotFound
				}
				if err := verifyEntry(ctx, v.repo, currentPolicy, currentAttestations, entry); err != nil {
					slog.Debug(fmt.Sprintf("Violation found: %s", err.Error()))
					slog.Debug("Checking if entry has been revoked...")
					// If the invalid entry is never marked as skipped, we return err
					if !entry.SkippedBy(annotations[entry.GetID().String()]) {
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
				} else if v.persistentCacheEnabled {
					// Verification has passed, add to cache
					v.persistentCache.SetLastVerifiedEntryForRef(entry.GetRefName(), entry.GetNumber(), entry.GetID())
				}

				continue
			}
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
		lastGoodEntry, lastGoodEntryAnnotations, err := rsl.GetLatestReferenceUpdaterEntry(v.repo, rsl.ForReference(invalidEntry.GetRefName()), rsl.BeforeEntryID(invalidEntry.GetID()), rsl.IsUnskipped(), rsl.IsReferenceEntry())
		if err != nil {
			return err
		}
		slog.Debug("Verifying identified last valid entry has not been revoked...")
		if lastGoodEntry.(*rsl.ReferenceEntry).SkippedBy(lastGoodEntryAnnotations) {
			// this type assertion is fine because we use the rsl.IsReferenceEntry opt
			return ErrLastGoodEntryIsSkipped
		}
		// require lastGoodEntry != nil

		// TODO: what if the very first entry for a ref is a violation?

		// gittuf requires the fix to point to a commit that is tree-same as the
		// last good state
		lastGoodTreeID, err := v.repo.GetCommitTreeID(lastGoodEntry.GetTargetID())
		if err != nil {
			return err
		}

		// 2. What entries do we have in the current verification set for the
		// ref? The first one that is tree-same as lastGoodEntry's commit is the
		// fix. Entries prior to that one in the queue are considered invalid
		// and must be skipped
		fixed := false
		var fixEntry *rsl.ReferenceEntry
		invalidIntermediateEntries := []*rsl.ReferenceEntry{}
		newEntryQueue := []rsl.ReferenceUpdaterEntry{}
		for len(entries) != 0 {
			newEntry := entries[0]
			entries = entries[1:]

			slog.Debug(fmt.Sprintf("Inspecting entry '%s' to see if it's a fix entry...", newEntry.GetID().String()))

			slog.Debug("Checking if entry is for the affected reference...")
			if newEntry.GetRefName() != invalidEntry.GetRefName() {
				// Unrelated entry that must be processed in the outer loop
				// Currently this is just policy entries
				newEntryQueue = append(newEntryQueue, newEntry)
				continue
			}

			switch newEntry := newEntry.(type) {
			case *rsl.PropagationEntry:
				// propagation entry cannot be a fix entry
				newEntryQueue = append(newEntryQueue, newEntry)
				continue

			case *rsl.ReferenceEntry:
				newCommitTreeID, err := v.repo.GetCommitTreeID(newEntry.GetTargetID())
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
						fixEntry = newEntry
						newEntryQueue = append(newEntryQueue, entries...)
					}
				}
				if fixed {
					break
				}

				// newEntry is not tree-same / commit-same, so it is automatically
				// invalid, check that it's been marked as revoked
				slog.Debug("Checking non-fix entry has been revoked as well...")
				if !newEntry.SkippedBy(annotations[newEntry.ID.String()]) {
					invalidIntermediateEntries = append(invalidIntermediateEntries, newEntry)
				}
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

		if v.persistentCacheEnabled {
			v.persistentCache.SetLastVerifiedEntryForRef(fixEntry.RefName, fixEntry.GetNumber(), fixEntry.GetID())
		}
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

	_, err = rootVerifier.Verify(ctx, gitinterface.ZeroHash, newPolicy.Metadata.RootEnvelope)
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
	slog.Debug("Searching for applicable reference authorizations and code reviews...")
	authorizationAttestation, approverKeyIDs, err := getApproverAttestationAndKeyIDs(ctx, repo, policy, attestationsState, entry)
	if err != nil {
		return err
	}

	// Verify Git namespace policies using the RSL entry and attestations
	if _, _, err := verifyGitObjectAndAttestations(ctx, policy, fmt.Sprintf("%s:%s", gitReferenceRuleScheme, entry.RefName), entry.ID, authorizationAttestation, withApproverPrincipalIDs(approverKeyIDs)); err != nil {
		return fmt.Errorf("verifying Git namespace policies failed, %w", ErrVerificationFailed)
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
			// If we've already verified and identified commit signature, we
			// can just check if that verifier is trusted for the new path.
			// If not found, we don't make any assumptions about it being a
			// failure in case of name mismatches. So, the signature check
			// proceeds as usual.
			verifiedUsing, _, err = verifyGitObjectAndAttestations(ctx, policy, fmt.Sprintf("%s:%s", fileRuleScheme, path), commitID, authorizationAttestation, withApproverPrincipalIDs(approverKeyIDs), withTrustedVerifier(verifiedUsing))
			if err != nil {
				return fmt.Errorf("verifying file namespace policies failed, %w", ErrVerificationFailed)
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

	if _, _, err := verifyGitObjectAndAttestations(ctx, policy, fmt.Sprintf("%s:%s", gitReferenceRuleScheme, entry.RefName), entry.GetID(), authorizationAttestation, withApproverPrincipalIDs(approverKeyIDs), withTagObjectID(entry.TargetID)); err != nil {
		return fmt.Errorf("verifying tag entry failed, %w: %w", ErrVerificationFailed, err)
	}

	return nil
}

func getApproverAttestationAndKeyIDs(ctx context.Context, repo *gitinterface.Repository, policy *State, attestationsState *attestations.Attestations, entry *rsl.ReferenceEntry) (*sslibdsse.Envelope, *set.Set[string], error) {
	if attestationsState == nil {
		return nil, nil, nil
	}

	firstEntry := false
	slog.Debug(fmt.Sprintf("Searching for RSL entry for '%s' before entry '%s'...", entry.RefName, entry.ID.String()))
	priorRefEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo, rsl.ForReference(entry.RefName), rsl.BeforeEntryID(entry.ID))
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil, nil, err
		}

		firstEntry = true
	}

	fromID := gitinterface.ZeroHash
	if !firstEntry {
		fromID = priorRefEntry.GetTargetID()
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

		githubApprovalAttestation, err := attestationsState.GetGitHubPullRequestApprovalAttestationFor(repo, policy.githubAppRoleName, targetRef, fromID.String(), toID.String())
		if err != nil {
			if !errors.Is(err, github.ErrPullRequestApprovalAttestationNotFound) {
				return nil, nil, err
			}
		}

		// if it exists
		if githubApprovalAttestation != nil {
			slog.Debug("GitHub pull request approval found, verifying attestation signature...")
			approvalVerifier := &SignatureVerifier{
				repository: policy.repository,
				name:       tuf.GitHubAppRoleName,
				principals: policy.githubAppKeys,
				threshold:  1, // TODO: support higher threshold
			}
			_, err := approvalVerifier.Verify(ctx, nil, githubApprovalAttestation)
			if err != nil {
				return nil, nil, fmt.Errorf("%w: failed to verify GitHub app approval attestation, signed by untrusted key", ErrVerificationFailed)
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

	priorRefEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo, rsl.ForReference(entry.RefName), rsl.BeforeEntryID(entry.ID))
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil, err
		}

		firstEntry = true
	}

	if firstEntry {
		return repo.GetCommitsBetweenRange(entry.TargetID, gitinterface.ZeroHash)
	}

	return repo.GetCommitsBetweenRange(entry.TargetID, priorRefEntry.GetTargetID())
}

// verifyGitObjectAndAttestationsOptions contains the configurable options for
// verifyGitObjectAndAttestations.
type verifyGitObjectAndAttestationsOptions struct {
	approverPrincipalIDs *set.Set[string]
	verifyMergeable      bool
	trustedVerifier      string
	tagObjectID          gitinterface.Hash
}

type verifyGitObjectAndAttestationsOption func(o *verifyGitObjectAndAttestationsOptions)

// withApproverPrincipalIDs allows for optionally passing in approver IDs to
// verifyGitObjectAndAttestations. These IDs may be obtained via a code review
// tool such as GitHub pull request approvals.
func withApproverPrincipalIDs(approverPrincipalIDs *set.Set[string]) verifyGitObjectAndAttestationsOption {
	return func(o *verifyGitObjectAndAttestationsOptions) {
		o.approverPrincipalIDs = approverPrincipalIDs
	}
}

// withVerifyMergeable indicates that the verification must check if a change
// can be merged.
func withVerifyMergeable() verifyGitObjectAndAttestationsOption {
	return func(o *verifyGitObjectAndAttestationsOptions) {
		o.verifyMergeable = true
	}
}

// withTrustedVerifier is used to specify the name of a verifier that has
// already been used to verify in the past. If the newly discovered set of
// verifiers includes the trusted verifier, then we can return early.
func withTrustedVerifier(name string) verifyGitObjectAndAttestationsOption {
	return func(o *verifyGitObjectAndAttestationsOptions) {
		o.trustedVerifier = name
	}
}

// withTagObjectID is used to set the Git ID of a tag object. When this is set,
// the tag object's signature is also verified in addition to the RSL entry for
// the tag.
func withTagObjectID(objID gitinterface.Hash) verifyGitObjectAndAttestationsOption {
	return func(o *verifyGitObjectAndAttestationsOptions) {
		o.tagObjectID = objID
	}
}

func verifyGitObjectAndAttestations(ctx context.Context, policy *State, target string, gitID gitinterface.Hash, authorizationAttestation *sslibdsse.Envelope, opts ...verifyGitObjectAndAttestationsOption) (string, bool, error) {
	options := &verifyGitObjectAndAttestationsOptions{tagObjectID: gitinterface.ZeroHash}
	for _, fn := range opts {
		fn(options)
	}

	verifiers, err := policy.FindVerifiersForPath(target)
	if err != nil {
		return "", false, err
	}

	if len(verifiers) == 0 {
		// This target is not protected by gittuf policy
		return "", false, nil
	}

	if options.trustedVerifier != "" {
		for _, verifier := range verifiers {
			if verifier.Name() == options.trustedVerifier {
				return options.trustedVerifier, false, nil
			}
		}
	}

	appName := ""
	if policy.githubAppApprovalsTrusted {
		appName = policy.githubAppRoleName
	}
	verifiedUsing, acceptedPrincipalIDs, rslSignatureNeededForThreshold, err := verifyGitObjectAndAttestationsUsingVerifiers(ctx, verifiers, gitID, authorizationAttestation, appName, options.approverPrincipalIDs, options.verifyMergeable)
	if err != nil {
		return "", false, err
	}

	if !options.tagObjectID.IsZero() {
		// Verify tag object's signature as well
		tagObjVerified := false
		for _, verifier := range verifiers {
			// explicitly not looking at the attestation
			// that applies to the _push_
			// thus, we also set threshold to 1
			verifier.threshold = 1

			_, err := verifier.Verify(ctx, options.tagObjectID, nil)
			if err == nil {
				// Signature verification succeeded
				tagObjVerified = true
				// TODO: should we check if a different verifier / signer was
				// matched for the tag object compared with the RSL entry?
				break
			} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
				// Unexpected error
				return "", false, err
			}
			// Haven't found a valid verifier, continue with next verifier
		}

		if !tagObjVerified {
			return "", false, fmt.Errorf("verifying tag object's signature failed")
		}
	}

	verifiedPrincipalIDs := 0
	if acceptedPrincipalIDs != nil {
		verifiedPrincipalIDs = acceptedPrincipalIDs.Len()
	}

	for controllerName, globalRules := range policy.globalRules {
		if controllerName == "" { // this is the special case
			slog.Debug("Checking global rules declared in current repository...")
		} else {
			slog.Debug(fmt.Sprintf("Checking global rules declared in controller repository '%s'...", controllerName))
		}
		for _, rule := range globalRules {
			// We check every global rule
			slog.Debug(fmt.Sprintf("Checking if global rule '%s' applies...", rule.GetName()))
			switch rule := rule.(type) {
			case tuf.GlobalRuleThreshold:
				if !rule.Matches(target) {
					break
				}

				// The global rule applies to the namespace under verification
				slog.Debug(fmt.Sprintf("Verifying threshold global rule '%s'...", rule.GetName()))
				requiredThreshold := rule.GetThreshold()
				if rslSignatureNeededForThreshold && options.verifyMergeable {
					// Since we're verifying if it's mergeable and we already know
					// that the RSL signature is needed to meet threshold, we can
					// reduce the global constraint threshold as well
					slog.Debug("Reducing required global threshold by 1 (verifying if change is mergeable and RSL signature is required)...")
					requiredThreshold--
				}
				if verifiedPrincipalIDs < requiredThreshold {
					// Check if the verifiedPrincipalIDs meets the required global
					// threshold
					slog.Debug(fmt.Sprintf("Global rule '%s' not met, required threshold '%d', only have '%d'", rule.GetName(), rule.GetThreshold(), verifiedPrincipalIDs))
					return "", false, ErrVerifierConditionsUnmet
				}

				slog.Debug(fmt.Sprintf("Successfully verified global rule '%s'", rule.GetName()))

			case tuf.GlobalRuleBlockForcePushes:
				// TODO: we use policy.repository, not ideal...
				if !rule.Matches(target) {
					break
				}

				// The global rule applies to the namespace under verification
				slog.Debug(fmt.Sprintf("Verifying block force pushes global rule '%s'...", rule.GetName()))

				if options.verifyMergeable {
					// Cannot check for force pushes for a proposed change
					slog.Debug("Cannot verify block force pushes global rule when verifying if a change is mergeable")
					break
				}

				// TODO: should we not look up the entry's afresh in the RSL here?
				// the in-memory cache _should_ make this okay, but something to
				// consider...

				// gitID _must_ be for an RSL reference entry, and we must find
				// its predecessor entry.
				// Why? Because the rule type only accepts git:<> as patterns.
				// If we have another object here, we've gone wrong somewhere.
				currentEntry, err := rsl.GetEntry(policy.repository, gitID)
				if err != nil {
					slog.Debug(fmt.Sprintf("unable to load RSL entry for '%s': %v", gitID.String(), err))
					return "", false, err
				}

				currentEntryRef, isReferenceEntry := currentEntry.(*rsl.ReferenceEntry)
				if !isReferenceEntry {
					slog.Debug(fmt.Sprintf("Expected '%s' to be RSL reference entry, aborting verification of block force pushes global rule...", gitID.String()))
					return "", false, rsl.ErrInvalidRSLEntry
				}

				previousEntryRef, _, err := rsl.GetLatestReferenceUpdaterEntry(policy.repository, rsl.BeforeEntryID(currentEntry.GetID()), rsl.ForReference(currentEntryRef.RefName), rsl.IsUnskipped())
				if err != nil {
					if errors.Is(err, rsl.ErrRSLEntryNotFound) {
						slog.Debug(fmt.Sprintf("Entry '%s' is the first one for reference '%s', cannot check if it's a force push", currentEntryRef.GetID().String(), currentEntryRef.RefName))
						break
					}

					return "", false, err
				}

				knows, err := policy.repository.KnowsCommit(currentEntryRef.TargetID, previousEntryRef.GetTargetID())
				if err != nil {
					return "", false, err
				}
				if !knows {
					slog.Debug(fmt.Sprintf("Current entry's commit '%s' is not a descendant of prior entry's commit '%s'", currentEntryRef.TargetID.String(), previousEntryRef.GetTargetID().String()))
					return "", false, ErrVerifierConditionsUnmet
				}

				slog.Debug(fmt.Sprintf("Successfully verified global rule '%s' as '%s' is a descendant of '%s'", rule.GetName(), currentEntryRef.TargetID.String(), previousEntryRef.GetTargetID().String()))

			default:
				slog.Debug("Unknown global rule type, aborting verification...")
				return "", false, tuf.ErrUnknownGlobalRuleType
			}
		}
	}

	return verifiedUsing, rslSignatureNeededForThreshold, nil
}

func verifyGitObjectAndAttestationsUsingVerifiers(ctx context.Context, verifiers []*SignatureVerifier, gitID gitinterface.Hash, authorizationAttestation *sslibdsse.Envelope, appName string, approverIDs *set.Set[string], verifyMergeable bool) (string, *set.Set[string], bool, error) {
	if len(verifiers) == 0 {
		return "", nil, false, ErrNoVerifiers
	}

	var (
		verifiedUsing                       string
		acceptedPrincipalIDs                *set.Set[string]
		rslEntrySignatureNeededForThreshold bool
	)
	for _, verifier := range verifiers {
		trustedPrincipalIDs := verifier.TrustedPrincipalIDs()

		usedPrincipalIDs, err := verifier.Verify(ctx, gitID, authorizationAttestation)
		if err == nil {
			// We meet requirements just from the authorization attestation's sigs
			verifiedUsing = verifier.Name()
			acceptedPrincipalIDs = usedPrincipalIDs
			break
		} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
			return "", nil, false, err
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
			acceptedPrincipalIDs = trustedUsedPrincipalIDs
			break
		}

		// If verifyMergeable is true, we only need to meet threshold - 1
		if verifyMergeable && verifier.Threshold() > 1 {
			if trustedUsedPrincipalIDs.Len() >= verifier.Threshold()-1 {
				slog.Debug(fmt.Sprintf("Counted '%d' principals towards threshold '%d' for '%s', policies can be met if the merge is by authorized person!", trustedUsedPrincipalIDs.Len(), verifier.Threshold(), verifier.Name()))
				verifiedUsing = verifier.Name()
				acceptedPrincipalIDs = trustedPrincipalIDs
				rslEntrySignatureNeededForThreshold = true
				break
			}
		}
	}

	if verifiedUsing != "" {
		return verifiedUsing, acceptedPrincipalIDs, rslEntrySignatureNeededForThreshold, nil
	}

	return "", nil, false, ErrVerifierConditionsUnmet
}
