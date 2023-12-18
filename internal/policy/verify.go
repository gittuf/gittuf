// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/common"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	sslibdsse "github.com/secure-systems-lab/go-securesystemslib/dsse"
)

const (
	nonCommitMessage                  = "cannot verify non-commit object"
	nonTagMessage                     = "cannot verify non-tag object"
	unableToResolveRevisionMessage    = "unable to resolve revision (must be a reference or a commit identifier)"
	noPublicKeyMessage                = "no public key found for Git object"
	unableToLoadPolicyMessageFmt      = "unable to load applicable gittuf policy: %s"
	unableToFindPolicyMessage         = "unable to find applicable gittuf policy"
	goodSignatureMessageFmt           = "good signature from key '%s:%s'"
	goodTagSignatureMessage           = "good signature for RSL entry and tag"
	goodSignatureMessageForRSLEntry   = "good signature for RSL entry"
	badSignatureMessageForRSLEntry    = "bad signature for RSL entry"
	noSignatureMessage                = "no signature found"
	errorVerifyingSignatureMessageFmt = "verifying signature using key '%s:%s' failed: %s"
	unableToFindRSLEntryMessage       = "unable to find tag's RSL entry"
	multipleTagRSLEntriesFoundMessage = "multiple RSL entries found for tag"
)

var (
	ErrUnauthorizedSignature   = errors.New("unauthorized signature")
	ErrInvalidEntryNotSkipped  = errors.New("invalid entry found not marked as skipped")
	ErrLastGoodEntryIsSkipped  = errors.New("entry expected to be unskipped is marked as skipped")
	ErrUnknownObjectType       = errors.New("unknown object type passed to verify signature")
	ErrInvalidVerifier         = errors.New("verifier has invalid parameters (is threshold 0?)")
	ErrVerifierConditionsUnmet = errors.New("verifier's key and threshold constrains not met")
)

// VerifyRef verifies the signature on the latest RSL entry for the target ref
// using the latest policy. The expected Git ID for the ref in the latest RSL
// entry is returned if the policy verification is successful.
func VerifyRef(ctx context.Context, repo *git.Repository, target string) (plumbing.Hash, error) {
	// 1. Get latest policy entry
	policyEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, PolicyRef)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	policyState, err := LoadStateForEntry(ctx, repo, policyEntry)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	// 2. Find latest entry for target
	latestEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, target)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return latestEntry.TargetID, verifyEntry(ctx, repo, policyState, latestEntry)
}

// VerifyRefFull verifies the entire RSL for the target ref from the first
// entry. The expected Git ID for the ref in the latest RSL entry is returned if
// the policy verification is successful.
func VerifyRefFull(ctx context.Context, repo *git.Repository, target string) (plumbing.Hash, error) {
	// 1. Trace RSL back to the start
	firstEntry, _, err := rsl.GetFirstEntry(repo)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	// 2. Find latest entry for target
	latestEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, target)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	// 3. Do a relative verify from start entry to the latest entry (firstEntry here == policyEntry)
	return latestEntry.TargetID, VerifyRelativeForRef(ctx, repo, firstEntry, firstEntry, latestEntry, target)
}

// VerifyRelativeForRef verifies the RSL between specified start and end entries
// using the provided policy entry for the first entry.
//
// TODO: should the policy entry be inferred from the specified first entry?
func VerifyRelativeForRef(ctx context.Context, repo *git.Repository, initialPolicyEntry, firstEntry, lastEntry *rsl.ReferenceEntry, target string) error {
	var currentPolicy *State

	// 1. Load policy applicable at firstEntry
	state, err := LoadStateForEntry(ctx, repo, initialPolicyEntry)
	if err != nil {
		return err
	}
	currentPolicy = state

	// 2. Enumerate RSL entries between firstEntry and lastEntry, ignoring irrelevant ones
	entries, annotations, err := rsl.GetReferenceEntriesInRangeForRef(repo, firstEntry.ID, lastEntry.ID, target)
	if err != nil {
		return err
	}

	// 3. Verify each entry, looking for a fix when an invalid entry is encountered
	var invalidEntry *rsl.ReferenceEntry
	var verificationErr error
	for len(entries) != 0 {
		if invalidEntry == nil {
			// Pop entry from queue
			entry := entries[0]
			entries = entries[1:]

			if entry.RefName == PolicyRef {
				// TODO: this is repetition if the firstEntry is for policy
				newPolicy, err := LoadStateForEntry(ctx, repo, entry)
				if err != nil {
					return err
				}

				if err := currentPolicy.VerifyNewState(ctx, newPolicy); err != nil {
					return err
				}

				currentPolicy = newPolicy
				continue
			}

			if err := verifyEntry(ctx, repo, currentPolicy, entry); err != nil {
				// If the invalid entry is never marked as skipped, we return err
				if !entry.SkippedBy(annotations[entry.ID]) {
					return err
				}

				// The invalid entry's been marked as skipped but we still need
				// to see if another entry fixed state for non-gittuf users
				invalidEntry = entry
				verificationErr = err
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
		lastGoodEntry, lastGoodEntryAnnotations, err := rsl.GetLatestUnskippedReferenceEntryForRefBefore(repo, invalidEntry.RefName, invalidEntry.ID)
		if err != nil {
			return err
		}
		if lastGoodEntry.SkippedBy(lastGoodEntryAnnotations) {
			return ErrLastGoodEntryIsSkipped
		}
		lastGoodEntryCommit, err := gitinterface.GetCommit(repo, lastGoodEntry.TargetID)
		if err != nil {
			return err
		}
		// gittuf requires the fix to point to a commit that is tree-same as the
		// last good state
		lastGoodTreeID := lastGoodEntryCommit.TreeHash

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

			if newEntry.RefName != invalidEntry.RefName {
				// Unrelated entry that must be processed in the outer loop
				// Currently this is just policy entries
				newEntryQueue = append(newEntryQueue, newEntry)
				continue
			}

			newEntryCommit, err := gitinterface.GetCommit(repo, newEntry.TargetID)
			if err != nil {
				return err
			}
			if newEntryCommit.TreeHash == lastGoodTreeID {
				// Fix found, we append the rest of the current verification set
				// to the new entry queue
				// But first, we must check that this fix hasn't been skipped
				// If it has been skipped, it's not actually a fix and we need
				// to keep looking
				if !newEntry.SkippedBy(annotations[newEntry.ID]) {
					fixed = true
					newEntryQueue = append(newEntryQueue, entries...)
					break
				}
			}

			// newEntry is not tree-same / commit-same, so it is automatically
			// invalid, check that it's been marked as revoked
			if !newEntry.SkippedBy(annotations[newEntry.ID]) {
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

// VerifyCommit verifies the signature on the specified commits (identified by
// their hash or via a reference that is resolved). For each commit, the policy
// applicable when the commit was first recorded (directly or indirectly) in the
// RSL is used. The function returns a map that identifies the verification
// status for each of the submitted IDs. All commit IDs that are passed in will
// have an entry in the returned status. The status is currently meant to be
// consumed directly by the user, as this is used for a special, user-invoked
// workflow. gittuf's other verification workflows are currently not expected to
// use this function.
func VerifyCommit(ctx context.Context, repo *git.Repository, ids ...string) map[string]string {
	status := make(map[string]string, len(ids))
	commits := make(map[string]*object.Commit, len(ids))

	for _, id := range ids {
		if gitinterface.IsTag(repo, id) {
			// we do this because ResolveRevision returns a tag's commit object.
			// For tags, we want to verify the signature on the tag object
			// rather than the underlying commit.
			status[id] = nonCommitMessage
			continue
		}

		rev, err := repo.ResolveRevision(plumbing.Revision(id))
		if err != nil {
			status[id] = unableToResolveRevisionMessage
			continue
		}
		commit, err := gitinterface.GetCommit(repo, *rev)
		if err != nil {
			if errors.Is(err, plumbing.ErrObjectNotFound) {
				status[id] = nonCommitMessage
			} else {
				status[id] = err.Error()
			}
			continue
		}
		commits[id] = commit
	}

	for id, commit := range commits {
		verified := false
		if len(commit.PGPSignature) == 0 {
			status[id] = noSignatureMessage
			continue
		}

		commitPolicy, err := GetStateForCommit(ctx, repo, commit)
		if err != nil {
			status[id] = fmt.Sprintf(unableToLoadPolicyMessageFmt, err.Error())
			continue
		}
		if commitPolicy == nil {
			status[id] = unableToFindPolicyMessage
			continue
		}

		// TODO: Add `applyFilePolicies` flag that uses the commitPolicy to
		// check that the commit signature is from a key trusted for all the
		// paths modified by the commit.

		keys, err := commitPolicy.PublicKeys()
		if err != nil {
			status[id] = fmt.Sprintf(unableToLoadPolicyMessageFmt, err.Error())
			continue
		}
		for _, key := range keys {
			err = gitinterface.VerifyCommitSignature(ctx, commit, key)
			if err == nil {
				verified = true
				status[id] = fmt.Sprintf(goodSignatureMessageFmt, key.KeyType, key.KeyID)
				break
			}

			if errors.Is(err, gitinterface.ErrUnknownSigningMethod) {
				// We encounter this for key types that can be used for gittuf
				// policy metadata but not Git objects
				continue
			}

			if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
				status[id] = fmt.Sprintf(errorVerifyingSignatureMessageFmt, key.KeyType, key.KeyID, err.Error())
			}
		}

		if !verified {
			status[id] = noPublicKeyMessage
		}
	}

	return status
}

// VerifyTag verifies the signature on the RSL entries for the specified tags.
// In addition, each tag object's signature is also verified using the same set
// of trusted keys. If the tag is not protected by policy, then all keys in the
// applicable policy are used to verify the signatures.
func VerifyTag(ctx context.Context, repo *git.Repository, ids []string) map[string]string {
	status := make(map[string]string, len(ids))

	for _, id := range ids {
		// Check if id is tag name or hash of tag obj
		absPath, err := gitinterface.AbsoluteReference(repo, id)
		if err == nil {
			if !strings.HasPrefix(absPath, gitinterface.TagRefPrefix) {
				status[id] = nonTagMessage
				continue
			}
		} else {
			if !errors.Is(err, gitinterface.ErrReferenceNotFound) {
				status[id] = err.Error()
				continue
			}

			// Must be a hash
			// verifyTagEntry also finds the tag object, wasteful?
			tagObj, err := gitinterface.GetTag(repo, plumbing.NewHash(id))
			if err != nil {
				status[id] = nonTagMessage
				continue
			}
			absPath = string(plumbing.NewTagReferenceName(tagObj.Name))
		}

		entry, _, err := rsl.GetLatestReferenceEntryForRef(repo, absPath)
		if err != nil {
			status[id] = unableToFindRSLEntryMessage
			continue
		}

		if _, _, err := rsl.GetLatestReferenceEntryForRefBefore(repo, absPath, entry.GetID()); err == nil {
			status[id] = multipleTagRSLEntriesFoundMessage
			continue
		}

		policyEntry, _, err := rsl.GetLatestReferenceEntryForRefBefore(repo, PolicyRef, entry.ID)
		if err != nil {
			status[id] = fmt.Sprintf(unableToLoadPolicyMessageFmt, err.Error())
			continue
		}

		policy, err := LoadStateForEntry(ctx, repo, policyEntry)
		if err != nil {
			status[id] = fmt.Sprintf(unableToLoadPolicyMessageFmt, err.Error())
			continue
		}

		if err := verifyTagEntry(ctx, repo, policy, entry); err == nil {
			status[id] = goodTagSignatureMessage
		} else {
			status[id] = err.Error()
		}
	}

	return status
}

// VerifyNewState ensures that when a new policy is encountered, its root role
// is signed by keys trusted in the current policy.
func (s *State) VerifyNewState(ctx context.Context, newPolicy *State) error {
	currentRoot, err := s.GetRootMetadata()
	if err != nil {
		return err
	}

	rootKeyIDs := currentRoot.Roles[RootRoleName].KeyIDs
	rootThreshold := currentRoot.Roles[RootRoleName].Threshold

	verifiers := make([]sslibdsse.Verifier, 0, len(rootKeyIDs))
	for _, keyID := range rootKeyIDs {
		k, ok := currentRoot.Keys[keyID]
		if !ok {
			// This is almost certainly an issue but we can be a little
			// permissive and let failure happen in case the threshold isn't
			// met
			continue
		}

		sv, err := signerverifier.NewSignerVerifierFromTUFKey(k)
		if err != nil {
			return err
		}

		verifiers = append(verifiers, sv)
	}

	return dsse.VerifyEnvelope(ctx, newPolicy.RootEnvelope, verifiers, rootThreshold)
}

// verifyEntry is a helper to verify an entry's signature using the specified
// policy. The specified policy is used for the RSL entry itself. However, for
// commit signatures, verifyEntry checks when the commit was first introduced
// via the RSL across all refs. Then, it uses the policy applicable at the
// commit's first entry into the repository. If the commit is brand new to the
// repository, the specified policy is used.
func verifyEntry(ctx context.Context, repo *git.Repository, policy *State, entry *rsl.ReferenceEntry) error {
	// TODO: discuss how / if we want to verify RSL entry signatures for the policy namespace
	if entry.RefName == PolicyRef {
		return nil
	}

	if strings.HasPrefix(entry.RefName, gitinterface.TagRefPrefix) {
		return verifyTagEntry(ctx, repo, policy, entry)
	}

	var (
		gitNamespaceVerified  = false
		pathNamespaceVerified = true // Assume paths are verified until we find out otherwise
	)

	// 1. Find authorized verifiers for entry's ref
	verifiers, err := policy.FindVerifiersForPath(ctx, fmt.Sprintf("git:%s", entry.RefName)) // FIXME: "git:" shouldn't be here
	if err != nil {
		return err
	}

	// No verifiers => no restrictions for the git namespace
	if len(verifiers) == 0 {
		gitNamespaceVerified = true
	}

	// 2. Find commit object for the RSL entry
	commitObj, err := gitinterface.GetCommit(repo, entry.ID)
	if err != nil {
		return err
	}

	// 3. Use each verifier to verify signature
	for _, verifier := range verifiers {
		err := verifier.Verify(ctx, commitObj, nil)
		if err == nil {
			// Signature verification succeeded
			gitNamespaceVerified = true
			break
		} else if !errors.Is(err, ErrVerifierConditionsUnmet) {
			// Unexpected error
			return err
		}
		// Haven't found a valid verifier, continue with next
	}

	if !gitNamespaceVerified {
		return fmt.Errorf("verifying Git namespace policies failed, %w", ErrUnauthorizedSignature)
	}

	// 4. Verify modified files

	// First, get all commits between the current and last entry for the ref.
	commits, err := getCommits(repo, entry) // note: this is ordered by commit ID
	if err != nil {
		return err
	}

	commitsVerified := make([]bool, len(commits))
	for i, commit := range commits {
		// Assume the commit's paths are verified, if a path is left unverified,
		// we flip this later.
		commitsVerified[i] = true

		paths, err := gitinterface.GetFilePathsChangedByCommit(repo, commit)
		if err != nil {
			return err
		}

		var commitPolicy *State
		commitPolicy, err = GetStateForCommit(ctx, repo, commit)
		if err != nil {
			return err
		}
		if commitPolicy == nil {
			// the commit hasn't been seen in any refs in the repository, use
			// specified policy
			commitPolicy = policy
		}

		pathsVerified := make([]bool, len(paths))
		verifiedUsing := "" // this will be set after one successful verification of the commit to avoid repeated signature verification
		for j, path := range paths {
			verifiers, err := commitPolicy.FindVerifiersForPath(ctx, fmt.Sprintf("file:%s", path)) // FIXME: "file:" shouldn't be here
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
				//
				// FIXME: this is probably a vuln as a rule name may re-occur
				// without being met by a target delegation in different
				// policies
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
				err := verifier.Verify(ctx, commit, nil)
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

func verifyTagEntry(ctx context.Context, repo *git.Repository, policy *State, entry *rsl.ReferenceEntry) error {
	// 1. Find authorized public keys for tag's RSL entry
	trustedKeys, err := policy.FindPublicKeysForPath(ctx, fmt.Sprintf("git:%s", entry.RefName))
	if err != nil {
		return err
	}

	if len(trustedKeys) == 0 {
		allKeys, err := policy.PublicKeys()
		if err != nil {
			return err
		}

		// FIXME: decide if we want to pass around map or slice for these APIs
		for _, key := range allKeys {
			trustedKeys = append(trustedKeys, key)
		}
	}

	// 2. Find commit object for the RSL entry
	commitObj, err := gitinterface.GetCommit(repo, entry.ID)
	if err != nil {
		return err
	}

	// 3. Use each trusted key to verify signature
	rslEntryVerified := false
	for _, key := range trustedKeys {
		err := gitinterface.VerifyCommitSignature(ctx, commitObj, key)
		if err == nil {
			// Signature verification succeeded
			rslEntryVerified = true
			break
		}
		if errors.Is(err, gitinterface.ErrUnknownSigningMethod) {
			// We encounter this for key types that can be used for gittuf
			// policy metadata but not Git objects
			continue
		}
		if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
			// Unexpected error
			return err
		}
		// Haven't found a valid key, continue with next key
	}

	if !rslEntryVerified {
		return fmt.Errorf("verifying RSL entry failed, %w", ErrUnauthorizedSignature)
	}

	// 4. Verify tag object
	tagObjVerified := false
	tagObj, err := gitinterface.GetTag(repo, entry.TargetID)
	if err != nil {
		// Likely indicates the ref is not pointing to a tag object
		// What about lightweight tags?
		return err
	}

	if len(tagObj.PGPSignature) == 0 {
		return fmt.Errorf(noSignatureMessage)
	}

	for _, key := range trustedKeys {
		err := gitinterface.VerifyTagSignature(ctx, tagObj, key)
		if err == nil {
			// Signature verification succeeded
			tagObjVerified = true
			break
		}
		if errors.Is(err, gitinterface.ErrUnknownSigningMethod) {
			// We encounter this for key types that can be used for gittuf
			// policy metadata but not Git objects
			continue
		}
		if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
			// Unexpected error
			return err
		}
		// Haven't found a valid key, continue with next key
	}

	if !tagObjVerified {
		return fmt.Errorf("verifying tag object's signature failed, %w", ErrUnauthorizedSignature)
	}

	return nil
}

// getCommits identifies the commits introduced to the entry's ref since the
// last RSL entry for the same ref. These commits are then verified for file
// policies.
func getCommits(repo *git.Repository, entry *rsl.ReferenceEntry) ([]*object.Commit, error) {
	firstEntry := false

	priorRefEntry, _, err := rsl.GetLatestReferenceEntryForRefBefore(repo, entry.RefName, entry.ID)
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil, err
		}

		firstEntry = true
	}

	if firstEntry {
		return gitinterface.GetCommitsBetweenRange(repo, entry.TargetID, plumbing.ZeroHash)
	}

	return gitinterface.GetCommitsBetweenRange(repo, entry.TargetID, priorRefEntry.TargetID)
}

// getChangedPaths identifies the paths of all the files changed using the
// specified RSL entry. The entry's commit ID is compared with the commit ID
// from the previous RSL entry for the same namespace.
//
// Deprecated: this was introduced in a previous design. As it turns out, it is
// flawed as we want changed paths per commit rather than all changed paths
// between two RSL entries that span multiple commits.
func getChangedPaths(repo *git.Repository, entry *rsl.ReferenceEntry) ([]string, error) {
	firstEntry := false

	currentCommit, err := gitinterface.GetCommit(repo, entry.TargetID)
	if err != nil {
		return nil, err
	}

	priorRefEntry, _, err := rsl.GetLatestReferenceEntryForRefBefore(repo, entry.RefName, entry.ID)
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil, err
		}

		firstEntry = true
	}

	if firstEntry {
		return gitinterface.GetCommitFilePaths(currentCommit)
	}

	priorCommit, err := gitinterface.GetCommit(repo, priorRefEntry.TargetID)
	if err != nil {
		return nil, err
	}

	return gitinterface.GetDiffFilePaths(currentCommit, priorCommit)
}

type Verifier struct {
	name      string
	keys      []*tuf.Key
	threshold int
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
// signature and in-toto attestation signatures.
func (v *Verifier) Verify(ctx context.Context, gitObject object.Object, attestation *sslibdsse.Envelope) error {
	if v.threshold < 1 {
		return ErrInvalidVerifier
	}

	if gitObject == nil && attestation == nil {
		return ErrVerifierConditionsUnmet
	}

	if attestation != nil {
		if (1 + len(attestation.Signatures)) < v.threshold {
			// Combining the attestation and the git object we still do not have
			// sufficient signatures
			return ErrVerifierConditionsUnmet
		}
	}

	var keyIDUsed string
	gitObjectVerified := false

	// First, verify the gitObject's signature if one is presented
	if gitObject != nil {
		switch o := gitObject.(type) {
		case *object.Commit:
			for _, key := range v.keys {
				err := gitinterface.VerifyCommitSignature(ctx, o, key)
				if err == nil {
					// Signature verification succeeded
					keyIDUsed = key.KeyID
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
		case *object.Tag:
			for _, key := range v.keys {
				err := gitinterface.VerifyTagSignature(ctx, o, key)
				if err == nil {
					// Signature verification succeeded
					keyIDUsed = key.KeyID
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
		default:
			return ErrUnknownObjectType
		}
	}

	// If threshold is 1 and the Git signature is verified, we can return
	if v.threshold == 1 && gitObjectVerified {
		return nil
	}

	// Second, verify signatures on the attestation, subtracting the threshold
	// by 1 to account for a verified git signature
	envelopeThreshold := v.threshold
	if gitObjectVerified {
		envelopeThreshold--
	}

	verifiers := make([]sslibdsse.Verifier, 0, len(v.keys)-1)
	for _, key := range v.keys {
		if key.KeyID == keyIDUsed {
			// Do not create a DSSE verifier for the key used to verify the Git
			// signature
			continue
		}

		verifier, err := signerverifier.NewSignerVerifierFromTUFKey(key)
		if err != nil && !errors.Is(err, common.ErrUnknownKeyType) {
			return err
		}
		verifiers = append(verifiers, verifier)
	}

	if err := dsse.VerifyEnvelope(ctx, attestation, verifiers, envelopeThreshold); err != nil {
		return ErrVerifierConditionsUnmet
	}

	return nil
}
