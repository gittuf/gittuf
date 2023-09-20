package gitinterface

import (
	"context"
	"errors"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
)

const DefaultRemoteName = "origin"

// PushRefSpec pushes from repo to the specified remote using pre-constructed
// refspecs. For more information on the Git refspec, please consult:
// https://git-scm.com/book/en/v2/Git-Internals-The-Refspec.
//
// All pushes are set to be atomic as the intent of using multiple refs is to
// sync the RSL.
func PushRefSpec(ctx context.Context, repo *git.Repository, remoteName string, refs []config.RefSpec) error {
	remote, err := repo.Remote(remoteName)
	if err != nil {
		return err
	}

	pushOpts := &git.PushOptions{
		RemoteName: remoteName,
		RefSpecs:   refs,
		Atomic:     true,
	}

	err = remote.PushContext(ctx, pushOpts)
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// Push constructs refspecs for the specified Git refs and pushes from the repo
// to the remote. For more information on the Git refspec, please consult:
// https://git-scm.com/book/en/v2/Git-Internals-The-Refspec.
//
// The refspecs are constructed to be fast-forward only.
func Push(ctx context.Context, repo *git.Repository, remoteName string, refs []string) error {
	refSpecs := make([]config.RefSpec, 0, len(refs))
	for _, r := range refs {
		refSpec, err := RefSpec(repo, r, "", true)
		if err != nil {
			return err
		}
		refSpecs = append(refSpecs, refSpec)
	}

	return PushRefSpec(ctx, repo, remoteName, refSpecs)
}

// Pull fetches the specified Git refs and gittuf namespaces and applies their
// changes to the local refs. If one of the specified refs is the current HEAD,
// the worktree is updated to reflect changes to that ref.
func Pull(ctx context.Context, repo *git.Repository, remoteName string, refs []string) error {
	absRefs := make([]string, 0, len(refs))
	for _, refName := range refs {
		absRef, err := AbsoluteReference(repo, refName)
		if err != nil {
			return err
		}
		absRefs = append(absRefs, absRef)
	}

	// 1. Fetch remote trackers
	if err := Fetch(ctx, repo, remoteName, refs, true); err != nil {
		return err
	}

	// 2. Check and set local references
	// Also check if head is in the list of refs pulled to update worktree
	headRef, err := repo.Reference(plumbing.HEAD, false)
	if err != nil {
		return err
	}
	var newHeadCommit plumbing.Hash
	for _, refName := range absRefs {
		ref, err := repo.Reference(plumbing.ReferenceName(refName), true)
		if err != nil {
			if !errors.Is(err, plumbing.ErrReferenceNotFound) {
				return err
			}
			if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
				return err
			}
			ref, err = repo.Reference(plumbing.ReferenceName(refName), true)
			if err != nil {
				return err
			}
		}
		remoteRef, err := repo.Reference(plumbing.ReferenceName(RemoteRefForLocalRef(refName, remoteName)), true)
		if err != nil {
			return err
		}
		if ref.Hash() == remoteRef.Hash() {
			continue
		}
		newRef := plumbing.NewHashReference(plumbing.ReferenceName(refName), remoteRef.Hash())
		if err := repo.Storer.CheckAndSetReference(newRef, ref); err != nil {
			return err
		}
		if refName == headRef.Target().String() {
			newHeadCommit = remoteRef.Hash()
		}
	}

	// 3. MergeReset worktree if HEAD was pulled
	if newHeadCommit.IsZero() {
		return nil
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}
	// FIXME: this isn't the correct merge behaviour for divergent branches,
	// go-git doesn't handle merges. Should we just shell out?
	return wt.Reset(&git.ResetOptions{
		Commit: newHeadCommit,
		Mode:   git.MergeReset,
	})
}

// FetchRefSpec fetches to the repo from the specified remote using
// pre-constructed refspecs. For more information on the Git refspec, please
// consult: https://git-scm.com/book/en/v2/Git-Internals-The-Refspec.
func FetchRefSpec(ctx context.Context, repo *git.Repository, remoteName string, refs []config.RefSpec) error {
	remote, err := repo.Remote(remoteName)
	if err != nil {
		return err
	}

	fetchOpts := &git.FetchOptions{
		RemoteName: remoteName,
		RefSpecs:   refs,
	}

	err = remote.FetchContext(ctx, fetchOpts)
	if err != nil {
		if !errors.Is(err, git.NoErrAlreadyUpToDate) {
			return err
		}
	}
	return nil
}

// Fetch constructs refspecs for the refs and fetches to the repo from the
// specified remote. For more information on the Git refspec, please consult:
// https://git-scm.com/book/en/v2/Git-Internals-The-Refspec.
//
// The refspecs are constructed to allow non-fast-forward fetches. Additionally,
// trackRemote controls if the change is fetched to a local remote tracker.
func Fetch(ctx context.Context, repo *git.Repository, remoteName string, refs []string, trackRemote bool) error {
	refSpecs := make([]config.RefSpec, 0, len(refs))
	for _, r := range refs {
		// We always update the remote tracker
		refSpec, err := RefSpec(repo, r, remoteName, false)
		if err != nil {
			return err
		}
		refSpecs = append(refSpecs, refSpec)

		// If we're not _exclusively_ tracking the remote, we also fetch to the
		// local ref
		if !trackRemote {
			refSpec, err := RefSpec(repo, r, "", false)
			if err != nil {
				return err
			}
			refSpecs = append(refSpecs, refSpec)
		}
	}

	return FetchRefSpec(ctx, repo, remoteName, refSpecs)
}

// CloneAndFetch clones a repository using the specified URL and additionally
// fetches the specified refs. If trackRemote is set, the refs are fetched to a
// local remote tracker.
func CloneAndFetch(ctx context.Context, remoteURL, dir, initialBranch string, refs []string, trackRemote bool) (*git.Repository, error) {
	repo, err := git.PlainCloneContext(ctx, dir, false, createCloneOptions(remoteURL, initialBranch))
	if err != nil {
		return nil, err
	}

	return fetchRefs(ctx, repo, refs, trackRemote)
}

// CloneAndFetchToMemory clones an in-memory repository using the specified URL
// and additionally fetches the specified refs. If trackRemote is set, the refs
// are fetched to a local remote tracker.
func CloneAndFetchToMemory(ctx context.Context, remoteURL, initialBranch string, refs []string, trackRemote bool) (*git.Repository, error) {
	repo, err := git.CloneContext(ctx, memory.NewStorage(), memfs.New(), createCloneOptions(remoteURL, initialBranch))
	if err != nil {
		return nil, err
	}

	return fetchRefs(ctx, repo, refs, trackRemote)
}

func createCloneOptions(remoteURL, initialBranch string) *git.CloneOptions {
	cloneOptions := &git.CloneOptions{URL: remoteURL}
	if len(initialBranch) > 0 {
		cloneOptions.ReferenceName = plumbing.ReferenceName(initialBranch)
	}

	return cloneOptions
}

func fetchRefs(ctx context.Context, repo *git.Repository, refs []string, trackRemote bool) (*git.Repository, error) {
	if len(refs) > 0 {
		err := Fetch(ctx, repo, DefaultRemoteName, refs, trackRemote)
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}
