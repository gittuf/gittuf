package policy

import (
	"context"
	"errors"
	"fmt"

	"github.com/adityasaky/gittuf/internal/gitinterface"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/adityasaky/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
)

var (
	ErrUnauthorizedSignature = errors.New("unauthorized signature")
)

func VerifyRef(ctx context.Context, repo *git.Repository, target string) error {
	// 1. Trace RSL back to the start
	firstEntryTmp, err := rsl.GetFirstEntry(repo)
	if err != nil {
		return err
	}
	firstEntry := firstEntryTmp.(*rsl.Entry)

	// 2. Find latest entry for target
	latestEntryTmp, err := rsl.GetLatestEntryForRef(repo, target)
	if err != nil {
		return err
	}
	latestEntry := latestEntryTmp.(*rsl.Entry)

	// 3. Do a relative verify from start entry to the latest entry
	return VerifyRelativeForRef(ctx, repo, firstEntry, latestEntry, target)
}

func VerifyRelativeForRef(ctx context.Context, repo *git.Repository, firstEntry *rsl.Entry, lastEntry *rsl.Entry, target string) error {
	entryStack := []*rsl.Entry{lastEntry}

	var currentPolicy *State
	// 1. Identify policy applicable at firstEntry
	policyEntry, err := rsl.GetLatestEntryForRefBefore(repo, PolicyRef, firstEntry.ID)
	if err != nil {
		return err
	}
	state, err := LoadStateForEntry(ctx, repo, policyEntry)
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
	entryQueue := []*rsl.Entry{}
	for j := len(entryStack) - 1; j >= 0; j-- {
		// TODO: instead of reversing it, we can likely just protest this in
		// reverse order but the policy states are the issue
		entryQueue = append(entryQueue, entryStack[j])
	}

	for _, entry := range entryQueue {
		// FIXME: we're not verifying policy RSL entry signatures because
		// we need to establish how to fetch that info. There's some code right
		// now in State that sets up all keys listed in policies as applicable
		// signers for that state but it's unclear if that's the way to go. An
		// additional blocker is for managing special keys like root and targets
		// keys. RSL entry signatures are commit signatures. What do we do when
		// metadata is signed using other methods? Do we instead flip the script
		// and require metadata signatures to match git signing methods?
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

func verifyEntry(ctx context.Context, repo *git.Repository, policy *State, entry *rsl.Entry) error {
	var (
		trustedKeys []*tuf.Key
		err         error
	)

	// 1. Find authorized public keys for entry's ref
	if entry.RefName == PolicyRef {
		trustedKeys = policy.AllPublicKeys
	} else {
		trustedKeys, err = policy.FindPublicKeysForPath(ctx, fmt.Sprintf("git:%s", entry.RefName)) // FIXME: "git:" shouldn't be here
		if err != nil {
			return err
		}
	}

	commitObj, err := repo.CommitObject(entry.ID)
	if err != nil {
		return err
	}

	for _, key := range trustedKeys {
		if err := gitinterface.VerifyCommitSignature(commitObj, key); err != nil {
			if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
				return err
			}
		} else {
			return nil
		}
	}

	return ErrUnauthorizedSignature
}
