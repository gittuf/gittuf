package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	ErrUnauthorizedSignature = errors.New("unauthorized signature")
)

// VerifyRef verifies the signature on the latest RSL entry for the target ref
// using the latest policy.
func VerifyRef(ctx context.Context, repo *git.Repository, target string) error {
	// 1. Get latest policy entry
	policyEntry, err := rsl.GetLatestEntryForRef(repo, PolicyRef)
	if err != nil {
		return err
	}
	policyState, err := LoadStateForEntry(ctx, repo, policyEntry)
	if err != nil {
		return err
	}

	// 2. Find latest entry for target
	latestEntry, err := rsl.GetLatestEntryForRef(repo, target)
	if err != nil {
		return err
	}

	return verifyEntry(ctx, repo, policyState, latestEntry)
}

// VerifyRefFull verifies the entire RSL for the target ref from the first
// entry.
func VerifyRefFull(ctx context.Context, repo *git.Repository, target string) error {
	// 1. Trace RSL back to the start
	firstEntry, err := rsl.GetFirstEntry(repo)
	if err != nil {
		return err
	}

	// 2. Find latest entry for target
	latestEntry, err := rsl.GetLatestEntryForRef(repo, target)
	if err != nil {
		return err
	}

	// 3. Do a relative verify from start entry to the latest entry (firstEntry here == policyEntry)
	return VerifyRelativeForRef(ctx, repo, firstEntry, firstEntry, latestEntry, target)
}

// VerifyRelativeForRef verifies the RSL between specified start and end entries
// using the provided policy entry for the first entry.
//
// TODO: should the policy entry be inferred from the specified first entry?
func VerifyRelativeForRef(ctx context.Context, repo *git.Repository, initialPolicyEntry *rsl.Entry, firstEntry *rsl.Entry, lastEntry *rsl.Entry, target string) error {
	entryStack := []*rsl.Entry{lastEntry}

	var currentPolicy *State
	// 1. Load policy applicable at firstEntry
	state, err := LoadStateForEntry(ctx, repo, initialPolicyEntry)
	if err != nil {
		return err
	}
	currentPolicy = state

	// 2. Enumerate RSL entries between firstEntry and lastEntry, ignoring irrelevant ones
	iteratorEntry := lastEntry
	for {
		if iteratorEntry.GetID() == firstEntry.ID {
			break
		}

		parentEntryTmp, err := rsl.GetParentForEntry(repo, iteratorEntry)
		if err != nil {
			return err
		}
		parentEntry := parentEntryTmp.(*rsl.Entry) // TODO: handle annotations

		if parentEntry.RefName == target || parentEntry.RefName == PolicyRef {
			entryStack = append(entryStack, parentEntry)
		}

		iteratorEntry = parentEntry
	}

	// entryStack has a list of RSL entries in reverse order
	entryQueue := make([]*rsl.Entry, 0, len(entryStack))
	for j := len(entryStack) - 1; j >= 0; j-- {
		// We reverse the entries so that they are chronologically sorted. If we
		// process them in the reverse order, we have to go past each entry
		// anyway to find the last policy entry to use. It also makes the entry
		// processing easier to reason about.
		entryQueue = append(entryQueue, entryStack[j])
	}

	for _, entry := range entryQueue {
		// FIXME: we're not verifying policy RSL entry signatures because we
		// need to establish how to fetch that info. An additional blocker is
		// for managing special keys like root and targets keys. RSL entry
		// signatures are commit signatures. What do we do when metadata is
		// signed using other methods? Do we instead flip the script and require
		// metadata signatures to match git signing methods?
		if entry.RefName == PolicyRef {
			// TODO: this is repetition if the firstEntry is for policy
			state, err := LoadStateForEntry(ctx, repo, entry)
			if err != nil {
				return err
			}
			currentPolicy = state

			continue
		}

		if err := verifyEntry(ctx, repo, currentPolicy, entry); err != nil {
			return err
		}
	}

	return nil
}

// verifyEntry is a helper to verify an entry's signature using the specified
// policy. The specified policy is used for the RSL entry itself. However, for
// commit signatures, verifyEntry checks when the commit was first introduced
// via the RSL across all refs. Then, it uses the policy applicable at the
// commit's first entry into the repository. If the commit is brand new to the
// repository, the specified policy is used.
func verifyEntry(ctx context.Context, repo *git.Repository, policy *State, entry *rsl.Entry) error {
	// TODO: discuss how / if we want to verify RSL entry signatures for the policy namespace
	if entry.RefName == PolicyRef {
		return nil
	}

	var (
		trustedKeys           []*tuf.Key
		err                   error
		gitNamespaceVerified  bool = false
		pathNamespaceVerified bool = true // Assume paths are verified until we find out otherwise
	)

	// 1. Find authorized public keys for entry's ref
	trustedKeys, err = policy.FindPublicKeysForPath(ctx, fmt.Sprintf("git:%s", entry.RefName)) // FIXME: "git:" shouldn't be here
	if err != nil {
		return err
	}

	// No trusted keys => allow any key's signature for the git namespace
	if len(trustedKeys) == 0 {
		gitNamespaceVerified = true
	}

	// 2. Find commit object for the RSL entry
	commitObj, err := repo.CommitObject(entry.ID)
	if err != nil {
		return err
	}

	// 3. Use each trusted key to verify signature
	for _, key := range trustedKeys {
		err := gitinterface.VerifyCommitSignature(ctx, commitObj, key)
		if err == nil {
			// Signature verification succeeded
			gitNamespaceVerified = true
			break
		}
		if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
			// Unexpected error
			return err
		}
		// Haven't found a valid key, continue with next key
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

		// TODO: evaluate if this can be done once for the earliest commit in
		// the set being verified if we had them ordered.
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
		verifiedKeyID := "" // this will be set after one successful verification of the commit to avoid repeated signature verification
		for j, path := range paths {
			trustedKeys, err := commitPolicy.FindPublicKeysForPath(ctx, fmt.Sprintf("file:%s", path)) // FIXME: "file:" shouldn't be here
			if err != nil {
				return err
			}

			if len(trustedKeys) == 0 {
				pathsVerified[j] = true
				continue
			}

			if len(verifiedKeyID) > 0 {
				// We've already verified and identified commit signature's key
				// ID, we can just check if that key ID is trusted for the new
				// path.
				// If not found, we don't make any assumptions about it being a
				// failure in case of key ID mismatches. So, the signature check
				// proceeds as usual.
				for _, key := range trustedKeys {
					if key.KeyID == verifiedKeyID {
						pathsVerified[j] = true
						break
					}
				}
			}

			if pathsVerified[j] {
				continue
			}

			for _, key := range trustedKeys {
				err := gitinterface.VerifyCommitSignature(ctx, commit, key)
				if err == nil {
					// Signature verification succeeded
					pathsVerified[j] = true
					verifiedKeyID = key.KeyID
					break
				}
				if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
					// Unexpected error
					return err
				}
				// Haven't found a valid key, continue with next key
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

// getCommits identifies the commits introduced to the entry's ref since the
// last RSL entry for the same ref. These commits are then verified for file
// policies.
func getCommits(repo *git.Repository, entry *rsl.Entry) ([]*object.Commit, error) {
	firstEntry := false

	priorRefEntry, err := rsl.GetLatestEntryForRefBefore(repo, entry.RefName, entry.ID)
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil, err
		}

		firstEntry = true
	}

	if firstEntry {
		return gitinterface.GetCommitsBetweenRange(repo, entry.CommitID, plumbing.ZeroHash)
	}

	return gitinterface.GetCommitsBetweenRange(repo, entry.CommitID, priorRefEntry.CommitID)
}

// getChangedPaths identifies the paths of all the files changed using the
// specified RSL entry. The entry's commit ID is compared with the commit ID
// from the previous RSL entry for the same namespace.
//
// Deprecated: this was introduced in a previous design. As it turns out, it is
// flawed as we want changed paths per commit rather than all changed paths
// between two RSL entries that span multiple commits.
func getChangedPaths(repo *git.Repository, entry *rsl.Entry) ([]string, error) {
	firstEntry := false

	currentCommit, err := repo.CommitObject(entry.CommitID)
	if err != nil {
		return nil, err
	}

	priorRefEntry, err := rsl.GetLatestEntryForRefBefore(repo, entry.RefName, entry.ID)
	if err != nil {
		if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
			return nil, err
		}

		firstEntry = true
	}

	if firstEntry {
		return gitinterface.GetCommitFilePaths(repo, currentCommit)
	}

	priorCommit, err := repo.CommitObject(priorRefEntry.CommitID)
	if err != nil {
		return nil, err
	}

	return gitinterface.GetDiffFilePaths(repo, currentCommit, priorCommit)
}
