// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"errors"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
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
	if errors.Is(err, transport.ErrEmptyRemoteRepository) || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

// Fetch constructs refspecs for the refs and fetches to the repo from the
// specified remote. For more information on the Git refspec, please consult:
// https://git-scm.com/book/en/v2/Git-Internals-The-Refspec.
//
// The fastForwardOnly flag controls if the constructed refspec allows
// non-fast-forward fetches. The target of the refspec is the same as the
// requested ref. Also, the remote tracker for the ref is also always updated.
func Fetch(ctx context.Context, repo *git.Repository, remoteName string, refs []string, fastForwardOnly bool) error {
	refSpecs := make([]config.RefSpec, 0, len(refs)*2)
	for _, r := range refs {
		// Add the remote tracker destination
		refSpec, err := RefSpec(repo, r, remoteName, fastForwardOnly)
		if err != nil {
			return err
		}
		refSpecs = append(refSpecs, refSpec)

		// Add the regular destination
		refSpec, err = RefSpec(repo, r, "", fastForwardOnly)
		if err != nil {
			return err
		}
		refSpecs = append(refSpecs, refSpec)
	}

	return FetchRefSpec(ctx, repo, remoteName, refSpecs)
}

// CloneAndFetch clones a repository using the specified URL and additionally
// fetches the specified refs.
func CloneAndFetch(ctx context.Context, remoteURL, dir, initialBranch string, refs []string) (*git.Repository, error) {
	repo, err := git.PlainCloneContext(ctx, dir, false, createCloneOptions(remoteURL, initialBranch))
	if err != nil {
		return nil, err
	}

	return fetchRefs(ctx, repo, refs, true)
}

// CloneAndFetchToMemory clones an in-memory repository using the specified URL
// and additionally fetches the specified refs.
func CloneAndFetchToMemory(ctx context.Context, remoteURL, initialBranch string, refs []string) (*git.Repository, error) {
	repo, err := git.CloneContext(ctx, memory.NewStorage(), memfs.New(), createCloneOptions(remoteURL, initialBranch))
	if err != nil {
		return nil, err
	}

	return fetchRefs(ctx, repo, refs, true)
}

func createCloneOptions(remoteURL, initialBranch string) *git.CloneOptions {
	cloneOptions := &git.CloneOptions{URL: remoteURL}
	if len(initialBranch) > 0 {
		cloneOptions.ReferenceName = plumbing.ReferenceName(initialBranch)
	}

	return cloneOptions
}

func fetchRefs(ctx context.Context, repo *git.Repository, refs []string, fastForwardOnly bool) (*git.Repository, error) {
	if len(refs) > 0 {
		err := Fetch(ctx, repo, DefaultRemoteName, refs, fastForwardOnly)
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}
