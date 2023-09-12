package repository

import (
	"context"
	"errors"
	"fmt"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
)

// RecordRSLEntryForReference is the interface for the user to add an RSL entry
// for the specified Git reference.
func (r *Repository) RecordRSLEntryForReference(refName string, signCommit bool) error {
	absRefName, err := gitinterface.AbsoluteReference(r.r, refName)
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

// CheckRemoteRSLForUpdates checks if the RSL at the specified remote remote
// repository has updated in comparison with the local repository's RSL. This is
// done by fetching the remote RSL to the local repository's remote RSL tracker.
// If the remote RSL has been updated, this method also checks if the local and
// remote RSLs have diverged. In summary, the first return value indicates if
// there is an update and the second return value indicates if the two RSLs have
// diverged and need to be reconciled.
func (r *Repository) CheckRemoteRSLForUpdates(ctx context.Context, remoteName string) (bool, bool, error) {
	trackerRef := fmt.Sprintf(rsl.RSLRemoteTrackerRef, remoteName)
	rslRemoteRefSpec := []config.RefSpec{config.RefSpec(fmt.Sprintf("%s:%s", rsl.RSLRef, trackerRef))}
	if err := gitinterface.FetchRefSpec(ctx, r.r, remoteName, rslRemoteRefSpec); err != nil {
		if errors.Is(err, transport.ErrEmptyRemoteRepository) {
			// Check if remote is empty and exit appropriately
			return false, false, nil
		}
		return false, false, err
	}

	remoteRefState, err := r.r.Reference(plumbing.ReferenceName(trackerRef), true)
	if err != nil {
		return false, false, err
	}

	localRefState, err := r.r.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
	if err != nil {
		return false, false, err
	}

	// Check if local is nil and exit appropriately
	if localRefState.Hash().IsZero() {
		// Local RSL has not been populated but remote is not zero
		// So there are updates the local can pull
		return true, false, nil
	}

	// Check if equal and exit early if true
	if remoteRefState.Hash() == localRefState.Hash() {
		return false, false, nil
	}

	// Next, check if remote is ahead of local
	remoteCommit, err := r.r.CommitObject(remoteRefState.Hash())
	if err != nil {
		return false, false, err
	}
	localCommit, err := r.r.CommitObject(localRefState.Hash())
	if err != nil {
		return false, false, err
	}

	knows, err := gitinterface.KnowsCommit(r.r, remoteCommit.Hash, localCommit)
	if err != nil {
		return false, false, err
	}
	if knows {
		return true, false, nil
	}

	// If not ancestor, local may be ahead or they may have diverged
	// If remote is ancestor, only local is ahead, no updates
	// If remote is not ancestor, the two have diverged, local needs to pull updates
	knows, err = gitinterface.KnowsCommit(r.r, localCommit.Hash, remoteCommit)
	if err != nil {
		return false, false, err
	}
	if knows {
		return false, false, nil
	}
	return true, true, nil
}
