package repository

import (
	"github.com/adityasaky/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/plumbing"
)

func (r *Repository) RecordRSLEntryForReference(refName string, signCommit bool) error {
	absRefName, err := absoluteReference(r.r, refName)
	if err != nil {
		return err
	}

	ref, err := r.r.Reference(plumbing.ReferenceName(absRefName), true)
	if err != nil {
		return err
	}

	// TODO: add some guardrails here

	return rsl.NewEntry(absRefName, ref.Hash()).Commit(r.r, signCommit)
}
