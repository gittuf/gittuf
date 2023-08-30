package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
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
// policy.
func verifyEntry(ctx context.Context, repo *git.Repository, policy *State, entry *rsl.Entry) error {
	// TODO: discuss how / if we want to verify RSL entry signatures for the policy namespace
	if entry.RefName == PolicyRef {
		return nil
	}

	var (
		trustedKeys []*tuf.Key
		err         error
	)

	// 1. Find authorized public keys for entry's ref
	trustedKeys, err = policy.FindPublicKeysForPath(ctx, fmt.Sprintf("git:%s", entry.RefName)) // FIXME: "git:" shouldn't be here
	if err != nil {
		return err
	}

	// No trusted keys => no protection
	if len(trustedKeys) == 0 {
		return nil
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
			return nil
		}
		if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
			// Unexpected error
			return err
		}
		// Haven't found a valid key, continue with next key
	}

	// 4. Verify modified files
	// TODO

	// ensure the linter doesn't complain about unused helper
	getChangedPaths(repo, entry) //nolint: errcheck

	return ErrUnauthorizedSignature
}

// getChangedPaths identifies the paths of all the files changed using the
// specified RSL entry. The entry's commit ID is compared with the commit ID
// from the previous RSL entry for the same namespace.
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
