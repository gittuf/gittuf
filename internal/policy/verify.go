package policy

import (
	"context"
	"errors"
	"reflect"
	"sort"

	"github.com/adityasaky/gittuf/internal/gitinterface"
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/adityasaky/gittuf/internal/tuf"
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

type ruleSet struct {
	rules []keyThresholdVerifier
	keys  map[string]*tuf.Key
}

func newRuleSet() *ruleSet {
	return &ruleSet{rules: []keyThresholdVerifier{}, keys: map[string]*tuf.Key{}}
}

func (r *ruleSet) addRuleSet(keys []*tuf.Key, threshold int) {
	if len(keys) == 0 {
		return
	}

	keyIDs := make([]string, 0, len(keys))
	for _, k := range keys {
		keyIDs = append(keyIDs, k.KeyID)
		r.keys[k.KeyID] = k
	}
	sort.Slice(keyIDs, func(i, j int) bool {
		return keyIDs[i] < keyIDs[j]
	})

	exists := false

	for _, rule := range r.rules {
		if rule.equals(keyIDs) {
			exists = true
			if threshold > rule.threshold {
				rule.threshold = threshold
			}
			break
		}
	}

	if !exists {
		r.rules = append(r.rules, keyThresholdVerifier{
			keys:      keyIDs,
			threshold: threshold,
			verified:  false,
		})
	}
}

func (r *ruleSet) verified() bool {
	for _, rule := range r.rules {
		if !rule.verified {
			return false
		}
	}

	return true
}

type keyThresholdVerifier struct {
	keys      []string
	threshold int
	verified  bool
}

func (v *keyThresholdVerifier) equals(keys []string) bool {
	return reflect.DeepEqual(v.keys, keys)
}

// verifyEntry is a helper to verify an entry's signature using the specified
// policy.
func verifyEntry(ctx context.Context, repo *git.Repository, policy *State, entry *rsl.Entry) error {
	// TODO: discuss how / if we want to verify RSL entry signatures for the policy namespace
	if entry.RefName == PolicyRef {
		return nil
	}

	rules := newRuleSet()

	// Find commit object for the RSL entry
	entryCommitObj, err := repo.CommitObject(entry.ID)
	if err != nil {
		return err
	}

	changedPaths, err := getChangedPaths(repo, entry)
	if err != nil {
		return err
	}

	// Find authorized public keys for entry's ref and every modified path
	for _, path := range changedPaths {
		trustedKeys, err := policy.FindPublicKeysForPath(ctx, entry.RefName, path)
		if err != nil {
			return err
		}
		rules.addRuleSet(trustedKeys, 1) // TODO: threshold
	}

	// No trusted keys => no protection
	if len(rules.rules) == 0 {
		return nil
	}

	// Verify each rule is fulfilled
	for _, rule := range rules.rules {
		for _, keyID := range rule.keys {
			err := gitinterface.VerifyCommitSignature(ctx, entryCommitObj, rules.keys[keyID])
			if err == nil {
				// TODO: threshold
				// For now, rule is satisfied
				rule.verified = true
				break
			}

			if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
				// Unexpected error
				return err
			}

			// Haven't found a valid key, continue with next key
		}
	}

	if rules.verified() {
		return nil
	}

	// for _, key := range trustedKeys {
	// 	err := gitinterface.VerifyCommitSignature(commitObj, key)
	// 	if err == nil {
	// 		// Signature verification succeeded
	// 		return nil
	// 	}
	// 	if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
	// 		// Unexpected error
	// 		return err
	// 	}
	// 	// Haven't found a valid key, continue with next key
	// }

	return ErrUnauthorizedSignature
}

func getChangedPaths(repo *git.Repository, entry *rsl.Entry) ([]string, error) {
	firstEntry := false
	filePaths := []string{}

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
		filesIter, err := currentCommit.Files()
		if err != nil {
			if errors.Is(err, plumbing.ErrObjectNotFound) {
				// empty commit
				return nil, nil
			}

			return nil, err
		}

		filesIter.ForEach(func(f *object.File) error {
			filePaths = append(filePaths, f.Name)
			return nil
		})

		return filePaths, nil
	}

	priorCommit, err := repo.CommitObject(priorRefEntry.CommitID)
	if err != nil {
		return nil, err
	}
	priorTreeEmpty := false
	priorTree, err := repo.TreeObject(priorCommit.TreeHash)
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			priorTreeEmpty = true
		} else {
			return nil, err
		}
	}

	currentTreeEmpty := false
	currentTree, err := repo.TreeObject(currentCommit.TreeHash)
	if err != nil {
		if errors.Is(err, plumbing.ErrObjectNotFound) {
			currentTreeEmpty = true
		} else {
			return nil, err
		}
	}

	if currentTreeEmpty && priorTreeEmpty {
		return nil, nil
	} else if priorTreeEmpty {
		// return files from current tree
	} else if currentTreeEmpty {
		// return files from prior tree, everything got deleted
	}

	diffSet := map[string]bool{}
	changes, err := currentTree.Diff(priorTree)
	if err != nil {
		return nil, err
	}
	for _, c := range changes {
		diffSet[c.From.Name] = true
		diffSet[c.To.Name] = true
	}

	for path := range diffSet {
		filePaths = append(filePaths, path)
	}

	return filePaths, nil
}
