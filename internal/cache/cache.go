// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"encoding/json"
	"errors"
	"log/slog"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
)

const (
	Ref = "refs/local/gittuf/persistent-cache"

	persistentTreeEntryName = "persistentCache"

	policyRef = "refs/gittuf/policy" // this is copied from internal/policy to avoid an import cycle
)

var (
	ErrNoPersistentCache = errors.New("persistent cache not found")
	ErrEntryNotNumbered  = errors.New("one or more entries are not numbered")
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
	treeID, err := treeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{gitinterface.NewEntryBlob(persistentTreeEntryName, blobID)})
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

// PopulatePersistentCache scans the repository's RSL and generates a persistent
// local-only cache of policy and attestation entries. This makes subsequent
// verifications faster.
func PopulatePersistentCache(repo *gitinterface.Repository) error {
	persistent := &Persistent{
		PolicyEntries:      []RSLEntryIndex{},
		AttestationEntries: []RSLEntryIndex{},
	}

	iterator, err := rsl.GetLatestEntry(repo)
	if err != nil {
		return err
	}

	if iterator.GetNumber() == 0 {
		return ErrEntryNotNumbered
	}

	persistent.AddedAttestationsBeforeNumber = iterator.GetNumber()

	for {
		if iterator, isReferenceEntry := iterator.(*rsl.ReferenceEntry); isReferenceEntry {
			switch iterator.RefName {
			case policyRef:
				persistent.InsertPolicyEntryNumber(iterator.GetNumber(), iterator.GetID())
			case attestations.Ref:
				persistent.InsertAttestationEntryNumber(iterator.GetNumber(), iterator.GetID())
			}
		}

		iterator, err = rsl.GetParentForEntry(repo, iterator)
		if err != nil {
			if errors.Is(err, rsl.ErrRSLEntryNotFound) {
				break
			}

			return err
		}

		if iterator.GetNumber() == 0 {
			return ErrEntryNotNumbered
		}
	}

	return persistent.Commit(repo)
}

// LoadPersistentCache loads the persistent cache from the tip of the local ref.
// If an instance has already been loaded and a pointer has been stored in
// memory, that instance is returned.
func LoadPersistentCache(repo *gitinterface.Repository) (*Persistent, error) {
	slog.Debug("Loading persistent cache from disk...")

	commitID, err := repo.GetReference(Ref)
	if err != nil {
		if errors.Is(err, gitinterface.ErrReferenceNotFound) {
			// Persistent cache doesn't exist
			slog.Debug("Persistent cache does not exist")
			return nil, ErrNoPersistentCache
		}

		return nil, err
	}

	treeID, err := repo.GetCommitTreeID(commitID)
	if err != nil {
		return nil, err
	}

	allFiles, err := repo.GetAllFilesInTree(treeID)
	if err != nil {
		return nil, err
	}

	blobID, has := allFiles[persistentTreeEntryName]
	if !has {
		// Persistent cache doesn't seem to exist? This maybe warrants
		// an error but we may have more than one file here in future?
		slog.Debug("Persistent cache does not exist")
		return nil, ErrNoPersistentCache
	}

	blob, err := repo.ReadBlob(blobID)
	if err != nil {
		return nil, err
	}

	persistentCache := &Persistent{}
	if err := json.Unmarshal(blob, &persistentCache); err != nil {
		return nil, err
	}

	slog.Debug("Loaded persistent cache")
	return persistentCache, nil
}

// ResetPersistentCache deletes the local persistent cache ref.
func ResetPersistentCache(repo *gitinterface.Repository) error {
	err := repo.DeleteReference(Ref)
	if err != nil {
		if errors.Is(err, gitinterface.ErrReferenceNotFound) {
			return nil
		}
		return err
	}
	return nil
}

// RSLEntryIndex is essentially a tuple that maps RSL entry IDs to numbers. This
// may be expanded in future to include more information as needed.
type RSLEntryIndex struct {
	EntryID     string `json:"entryID"`
	EntryNumber uint64 `json:"entryNumber"`
}

func (r *RSLEntryIndex) GetEntryID() gitinterface.Hash {
	hash, _ := gitinterface.NewHash(r.EntryID)
	// TODO: error?
	return hash
}

func (r *RSLEntryIndex) GetEntryNumber() uint64 {
	return r.EntryNumber
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
