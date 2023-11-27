// SPDX-License-Identifier: Apache-2.0

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

var (
	ErrCommitNotInRef = errors.New("specified commit is not in ref")
	ErrPushingRSL     = errors.New("unable to push RSL")
	ErrPullingRSL     = errors.New("unable to pull RSL")
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

	return rsl.NewReferenceEntry(absRefName, ref.Hash()).Commit(r.r, signCommit)
}

// RecordRSLEntryForReferenceAtCommit is a special version of
// RecordRSLEntryForReference used for evaluation. It is only invoked when
// gittuf is explicitly set in eval mode. This interface adds an RSL entry for
// the specified Git reference at the specified commit. If the commit is not in
// that ref, an entry is not created.
func (r *Repository) RecordRSLEntryForReferenceAtCommit(refName string, commitID string, signCommit bool) error {
	absRefName, err := gitinterface.AbsoluteReference(r.r, refName)
	if err != nil {
		return err
	}

	ref, err := r.r.Reference(plumbing.ReferenceName(absRefName), true)
	if err != nil {
		return err
	}

	commit, err := gitinterface.GetCommit(r.r, plumbing.NewHash(commitID))
	if err != nil {
		return err
	}

	// Even in eval mode, we ought to only create RSL entries for commits
	// actually in the specified ref.
	if knows, err := gitinterface.KnowsCommit(r.r, ref.Hash(), commit); err != nil {
		return err
	} else if !knows {
		return ErrCommitNotInRef
	}

	// TODO: once policy verification is in place, the signing key used by
	// signCommit must be verified for the refName in the delegation tree.

	return rsl.NewReferenceEntry(absRefName, plumbing.NewHash(commitID)).Commit(r.r, signCommit)
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

	return rsl.NewAnnotationEntry(rslEntryHashes, skip, message).Commit(r.r, signCommit)
}

// CheckRemoteRSLForUpdates checks if the RSL at the specified remote remote
// repository has updated in comparison with the local repository's RSL. This is
// done by fetching the remote RSL to the local repository's remote RSL tracker.
// If the remote RSL has been updated, this method also checks if the local and
// remote RSLs have diverged. In summary, the first return value indicates if
// there is an update and the second return value indicates if the two RSLs have
// diverged and need to be reconciled.
func (r *Repository) CheckRemoteRSLForUpdates(ctx context.Context, remoteName string) (bool, bool, error) {
	trackerRef := rsl.RemoteTrackerRef(remoteName)
	rslRemoteRefSpec := []config.RefSpec{config.RefSpec(fmt.Sprintf("%s:%s", rsl.Ref, trackerRef))}
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

	localRefState, err := r.r.Reference(plumbing.ReferenceName(rsl.Ref), true)
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
	remoteCommit, err := gitinterface.GetCommit(r.r, remoteRefState.Hash())
	if err != nil {
		return false, false, err
	}
	localCommit, err := gitinterface.GetCommit(r.r, localRefState.Hash())
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

// PushRSL pushes the local RSL to the specified remote. As this push defaults
// to fast-forward only, divergent RSL states are detected.
func (r *Repository) PushRSL(ctx context.Context, remoteName string) error {
	if err := gitinterface.Push(ctx, r.r, remoteName, []string{rsl.Ref}); err != nil {
		return errors.Join(ErrPushingRSL, err)
	}

	return nil
}

// PullRSL pulls RSL contents from the specified remote to the local RSL. The
// fetch is marked as fast forward only to detect RSL divergence.
func (r *Repository) PullRSL(ctx context.Context, remoteName string) error {
	if err := gitinterface.Fetch(ctx, r.r, remoteName, []string{rsl.Ref}, true); err != nil {
		return errors.Join(ErrPullingRSL, err)
	}

	return nil
}
