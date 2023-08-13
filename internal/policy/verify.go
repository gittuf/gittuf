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
	"github.com/go-git/go-git/v5/plumbing/object"
)

var (
	ErrUnauthorizedSignature = errors.New("unauthorized signature")
	ErrInvalidSetOfEntries   = errors.New("collection of entries being verified must make the same claims")
	ErrNotAncestor           = errors.New("first entry is not ancestor of second entry")
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

	return verifyEntries(ctx, repo, policyState, []*rsl.Entry{latestEntry})
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
			if errors.Is(err, rsl.ErrRSLEntryNotFound) {
				return ErrNotAncestor
			}

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

		if err := verifyEntries(ctx, repo, currentPolicy, []*rsl.Entry{entry}); err != nil {
			return err
		}
	}

	return nil
}

// ruleSet defines the set of rules that must be satisfied during verification.
// A rule is defined by a set of public keys and a threshold.
type ruleSet struct {
	rules []*keyThresholdVerifier
	keys  map[string]*tuf.Key
}

// newRuleSet creates a new instance of ruleSet.
func newRuleSet() *ruleSet {
	return &ruleSet{rules: []*keyThresholdVerifier{}, keys: map[string]*tuf.Key{}}
}

// addRuleSet adds a new rule that must be satisfied to the rule set.
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

	// If a rule set exists with the same public keys, the higher of the two
	// thresholds is used to avoid repeated verification.
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
		r.rules = append(r.rules, &keyThresholdVerifier{
			keys:      keyIDs,
			threshold: threshold,
			verified:  false,
		})
	}
}

// verified indicates if all the rules in the ruleSet have been satisfied.
func (r *ruleSet) verified() bool {
	for _, rule := range r.rules {
		if !rule.verified {
			return false
		}
	}

	return true
}

// keyThresholdVerifier defines a single rule. A rule is a combination of a set
// of verification keys and a threshold.
type keyThresholdVerifier struct {
	keys      []string
	threshold int
	verified  bool
}

// equals indicates if a slice of verification key IDs match the rule.
func (v *keyThresholdVerifier) equals(keys []string) bool {
	return reflect.DeepEqual(v.keys, keys)
}

// verifyEntries is a helper to verify an entry's signature using the specified
// policy. Each content in the entries slice must be identical except for their
// signature.
func verifyEntries(ctx context.Context, repo *git.Repository, policy *State, entries []*rsl.Entry) error {
	// TODO: discuss how / if we want to verify RSL entry signatures for the policy namespace
	if entries[0].RefName == PolicyRef {
		return nil
	}

	rules := newRuleSet()

	// Find commit object for the RSL entries
	entryCommitObjs := make([]*object.Commit, 0, len(entries))
	for _, e := range entries {
		// In the process, we verify that all entries make the same claims
		if e.RefName != entries[0].RefName {
			return ErrInvalidSetOfEntries
		}

		if e.CommitID != entries[0].CommitID {
			return ErrInvalidSetOfEntries
		}

		entryCommitObj, err := repo.CommitObject(e.ID)
		if err != nil {
			return err
		}

		entryCommitObjs = append(entryCommitObjs, entryCommitObj)
	}

	changedPaths, err := getChangedPaths(repo, entries[0])
	if err != nil {
		return err
	}

	// Find authorized public keys for entry's ref and every modified path
	for _, path := range changedPaths {
		trustedKeys, err := policy.FindPublicKeysForPath(ctx, entries[0].RefName, path)
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
		verifiedCount := 0
		for _, keyID := range rule.keys {
			if rule.verified {
				// Allows us to break early if prior keys satisfied the rule
				break
			}

			for _, entryCommitObj := range entryCommitObjs {
				err := gitinterface.VerifyCommitSignature(ctx, entryCommitObj, rules.keys[keyID])
				if err == nil {
					verifiedCount++

					// If threshold is met, mark rule verified
					if verifiedCount >= rule.threshold {
						rule.verified = true
						break
					}
				}

				if !errors.Is(err, gitinterface.ErrIncorrectVerificationKey) {
					// Unexpected error
					return err
				}

				// Haven't found a valid key, continue with next key
			}
		}
	}

	if rules.verified() {
		return nil
	}

	return ErrUnauthorizedSignature
}

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
