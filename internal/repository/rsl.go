package repository

import (
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/plumbing"
)

// RecordRSLEntryForReference is the interface for the user to add an RSL entry
// for the specified Git reference.
func (r *Repository) RecordRSLEntryForReference(refName string, signCommit bool) error {
	absRefName, err := absoluteReference(r.r, refName)
	if err != nil {
		return err
	}

	ref, err := r.r.Reference(plumbing.ReferenceName(absRefName), true)
	if err != nil {
		return err
	}

	// TODO: once policy verification is in place, the signing key used by
	// signCommit must be verified for the refName in the delegation tree.

	return rsl.NewEntry(absRefName, ref.Hash()).Commit(r.r, signCommit)
}

// RecordRSLAnnotation is the interface for the user to add an RSL annotation
// for one or more prior RSL entries.
func (r *Repository) RecordRSLAnnotation(rslEntryIDs []string, skip bool, message string, signCommit bool) error {
	rslEntryHashes := []plumbing.Hash{}
	for _, id := range rslEntryIDs {
		rslEntryHashes = append(rslEntryHashes, plumbing.NewHash(id))
	}

	// TODO: once policy verification is in place, the signing key used by
	// signCommit must be verified for the refNames of the rslEntryIDs.

	return rsl.NewAnnotation(rslEntryHashes, skip, message).Commit(r.r, signCommit)
}
