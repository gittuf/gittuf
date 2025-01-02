// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"encoding/json"

	"github.com/gittuf/gittuf/internal/gitinterface"
)

const (
	Ref = "refs/local/gittuf/persistent-cache"

	persistentTreeEntryName = "persistentCache"
)

type Persistent struct {
	// PolicyEntries is a list of index values for entries pertaining to the
	// policy ref. The list is ordered by each entry's Number.
	PolicyEntries []RSLEntryIndex `json:"policyEntries"`

	// AttestationEntries is a list of index values for entries pertaining to
	// the attestations ref. The list is ordered by each entry's Number.
	AttestationEntries []RSLEntryIndex `json:"attestationEntries"`

	// AddedAttestationsBeforeNumber tracks the number up to which
	// attestations have been searched for and added to
	// attestationsEntryNumbers. We need to track this for attestations in
	// particular because attestations are optional in gittuf repositories,
	// meaning attestationsEntryNumbers may be empty which would trigger a
	// full search.
	AddedAttestationsBeforeNumber uint64 `json:"addedAttestationsBeforeNumber"`

	// LastVerifiedEntryForRef is a map that indicates the last verified RSL
	// entry for a ref.
	LastVerifiedEntryForRef map[string]RSLEntryIndex `json:"lastVerifiedEntryForRef"`
}

func (p *Persistent) Commit(repo *gitinterface.Repository) error {
	if len(p.PolicyEntries) == 0 && len(p.AttestationEntries) == 0 && p.AddedAttestationsBeforeNumber == 0 && len(p.LastVerifiedEntryForRef) == 0 {
		// nothing to do
		return nil
	}

	contents, err := json.Marshal(p)
	if err != nil {
		return err
	}

	blobID, err := repo.WriteBlob(contents)
	if err != nil {
		return err
	}

	treeBuilder := gitinterface.NewTreeBuilder(repo)
	treeID, err := treeBuilder.WriteRootTreeFromBlobIDs(map[string]gitinterface.Hash{persistentTreeEntryName: blobID})
	if err != nil {
		return err
	}

	currentCommitID, _ := repo.GetReference(Ref) //nolint:errcheck
	if !currentCommitID.IsZero() {
		currentTreeID, err := repo.GetCommitTreeID(currentCommitID)
		if err == nil && treeID.Equal(currentTreeID) {
			// no change in cache contents, noop
			return nil
		}
	}

	_, err = repo.Commit(treeID, Ref, "Set persistent cache\n", false)
	return err
}

// RSLEntryIndex is essentially a tuple that maps RSL entry IDs to numbers. This
// may be expanded in future to include more information as needed.
type RSLEntryIndex struct {
	entryID     gitinterface.Hash
	entryNumber uint64
}

func (r *RSLEntryIndex) GetEntryID() gitinterface.Hash {
	return r.entryID
}

func (r *RSLEntryIndex) GetEntryNumber() uint64 {
	return r.entryNumber
}

func binarySearch(a, b RSLEntryIndex) int {
	if a.GetEntryNumber() == b.GetEntryNumber() {
		// Exact match
		return 0
	}

	if a.GetEntryNumber() < b.GetEntryNumber() {
		// Precedes
		return -1
	}

	// Succeeds
	return 1
}
