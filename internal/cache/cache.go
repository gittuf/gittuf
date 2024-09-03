// SPDX-License-Identifier: Apache-2.0

package cache

import (
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"slices"

	"github.com/gittuf/gittuf/internal/gitinterface"
)

type persistentCacheCtxKey string

const (
	Ref                                      = "refs/local/gittuf-persistent"
	PersistentCacheKey persistentCacheCtxKey = "persistentCache"

	persistentTreeEntryName = "persistent"
)

var ErrInvalidEntry = errors.New("invalid cache entry")

type Persistent struct {
	// PolicyEntryNumbers is a list of EntryNumberToID values for entries
	// pertaining to the policy ref. The list is ordered by each entry's Number.
	PolicyEntryNumbers []EntryNumberToID

	// AttestationsEntryNumbers is a list of EntryNumberToID values for entries
	// pertaining to the attestations ref. The list is ordered by each entry's
	// Number.
	AttestationsEntryNumbers []EntryNumberToID

	AddedAttestationsBeforeNumber uint64

	// LastVerifiedEntryForRef is a map that indicates the last verified RSL
	// entry for a ref.
	LastVerifiedEntryForRef map[string]EntryNumberToID
}

func (p *Persistent) Commit(repo *gitinterface.Repository) error {
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

	_, err = repo.Commit(treeID, Ref, "Set persistent cache\n", false)
	return err
}

func (p *Persistent) HasPolicyEntryNumber(entryNumber uint64) (gitinterface.Hash, bool) {
	if len(p.PolicyEntryNumbers) == 0 || entryNumber == 0 {
		return gitinterface.ZeroHash, false
	}

	index, has := slices.BinarySearchFunc(p.PolicyEntryNumbers, EntryNumberToID{Number: entryNumber}, binarySearch)
	if !has {
		return gitinterface.ZeroHash, false
	}

	// Unlike Find... we're actively checking if a policy number has been
	// inserted into the cache before, so we return the ID from that index
	// exactly
	return p.PolicyEntryNumbers[index].ID, true
}

func (p *Persistent) FindPolicyEntryNumberForEntry(entryNumber uint64) EntryNumberToID {
	if len(p.PolicyEntryNumbers) == 0 {
		return EntryNumberToID{Number: 0}
	}

	index, has := slices.BinarySearchFunc(p.PolicyEntryNumbers, EntryNumberToID{Number: entryNumber}, binarySearch)
	if has {
		// The entry number given to us is the first entry which happens to be
		// the start of verification as well
		// This can happen for full verification
		return p.PolicyEntryNumbers[index]
	}

	// When !has, index is point of insertion, but we want the applicable entry
	// which is index-1
	return p.PolicyEntryNumbers[index-1]
}

func (p *Persistent) InsertPolicyEntryNumber(entryNumber uint64, entryID gitinterface.Hash) {
	if entryNumber == 0 {
		// For now, we don't have a way to track non-numbered entries
		return
	}

	slog.Debug(fmt.Sprintf("Inserting policy entry with ID '%s' and number %d into persistent cache...", entryID.String(), entryNumber))

	if len(p.PolicyEntryNumbers) == 0 {
		// No entries yet, just add the current entry
		p.PolicyEntryNumbers = []EntryNumberToID{{Number: entryNumber, ID: entryID}}
		return
	}

	if p.PolicyEntryNumbers[len(p.PolicyEntryNumbers)-1].Number < entryNumber {
		// Current entry clearly belongs at the very end
		p.PolicyEntryNumbers = append(p.PolicyEntryNumbers, EntryNumberToID{Number: entryNumber, ID: entryID})
		return
	}

	// We don't check the converse where the current entry is less than the
	// first entry because we're inserting as entries are encountered
	// chronologically. Worst case, binary search fallthrough below will still
	// handle it

	index, has := slices.BinarySearchFunc(p.PolicyEntryNumbers, EntryNumberToID{Number: entryNumber}, binarySearch)
	if has {
		// We could assume that if we've seen an entry with a number greater
		// than this, we should have seen this one too, but for now...
		return
	}

	newSlice := make([]EntryNumberToID, 0, len(p.PolicyEntryNumbers)+1)
	for i := 0; i < index; i++ {
		newSlice[i] = p.PolicyEntryNumbers[i]
	}
	for i := index + 1; i < len(p.PolicyEntryNumbers)+1; i++ {
		newSlice[i] = p.PolicyEntryNumbers[i-1]
	}
	newSlice[index] = EntryNumberToID{Number: entryNumber, ID: entryID}

	p.PolicyEntryNumbers = newSlice
}

func (p *Persistent) FindAttestationsEntryNumberForEntry(entryNumber uint64) EntryNumberToID {
	if len(p.AttestationsEntryNumbers) == 0 {
		return EntryNumberToID{Number: 0}
	}

	// Set entryNumber as max scanned if it's higher than what's already there
	if p.AddedAttestationsBeforeNumber < entryNumber {
		p.AddedAttestationsBeforeNumber = entryNumber
	}

	index, has := slices.BinarySearchFunc(p.AttestationsEntryNumbers, EntryNumberToID{Number: entryNumber}, binarySearch)
	if has {
		return p.AttestationsEntryNumbers[index]
	}

	return p.AttestationsEntryNumbers[index-1]
}

func (p *Persistent) InsertAttestationEntryNumber(entryNumber uint64, entryID gitinterface.Hash) {
	if entryNumber == 0 {
		// For now, we don't have a way to track non-numbered entries
		return
	}

	if len(p.AttestationsEntryNumbers) == 0 {
		// No entries yet, just add the current entry
		p.AttestationsEntryNumbers = []EntryNumberToID{{Number: entryNumber, ID: entryID}}
		return
	}

	if p.AttestationsEntryNumbers[len(p.AttestationsEntryNumbers)-1].Number < entryNumber {
		// Current entry clearly belongs at the very end
		p.AttestationsEntryNumbers = append(p.AttestationsEntryNumbers, EntryNumberToID{Number: entryNumber, ID: entryID})
		return
	}

	// We don't check the converse where the current entry is less than the
	// first entry because we're inserting as entries are encountered
	// chronologically. Worst case, binary search fallthrough below will still
	// handle it

	index, has := slices.BinarySearchFunc(p.AttestationsEntryNumbers, EntryNumberToID{Number: entryNumber}, binarySearch)
	if has {
		// We could assume that if we've seen an entry with a number greater
		// than this, we should have seen this one too, but for now...
		return
	}

	newSlice := make([]EntryNumberToID, 0, len(p.AttestationsEntryNumbers)+1)
	for i := 0; i < index; i++ {
		newSlice[i] = p.AttestationsEntryNumbers[i]
	}
	for i := index + 1; i < len(p.AttestationsEntryNumbers)+1; i++ {
		newSlice[i] = p.AttestationsEntryNumbers[i-1]
	}
	newSlice[index] = EntryNumberToID{Number: entryNumber, ID: entryID}

	p.AttestationsEntryNumbers = newSlice
}

func NewPersistentCache() *Persistent {
	return &Persistent{
		PolicyEntryNumbers:       []EntryNumberToID{},
		AttestationsEntryNumbers: []EntryNumberToID{},
		LastVerifiedEntryForRef:  map[string]EntryNumberToID{},
	}
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
			slog.Debug("Persistent cache does not exist, creating new instance...")
			return NewPersistentCache(), nil
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
		// Persistent cache doesn't seem to exist? This maybe warrants an
		// error but we may have more than one file here in future?
		slog.Debug("Persistent cache does not exist, creating new instance...")
		return NewPersistentCache(), nil
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

type EntryNumberToID struct {
	// Number is the RSL entry's number.
	Number uint64

	// ID is the RSL entry's Git ID.
	ID gitinterface.Hash
}

func binarySearch(a, b EntryNumberToID) int {
	if a.Number == b.Number {
		// Exact match
		return 0
	}

	if a.Number < b.Number {
		// Precedes
		return -1
	}

	// Succeeds
	return 1
}
