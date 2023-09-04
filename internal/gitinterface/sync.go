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

func Fetch(ctx context.Context, repo *git.Repository, remoteName string, refs []string, trackRemote bool) error {
	refSpecs := make([]config.RefSpec, 0, len(refs))
	for _, r := range refs {
		var (
			refSpec config.RefSpec
			err     error
		)
		if trackRemote {
			refSpec, err = RefSpec(repo, r, remoteName, false)
			if err != nil {
				return err
			}
		} else {
			refSpec, err = RefSpec(repo, r, "", false)
			if err != nil {
				return err
			}
		}
		refSpecs = append(refSpecs, refSpec)
	}

	return FetchRefSpec(ctx, repo, remoteName, refSpecs)
}

func CloneAndFetch(ctx context.Context, remoteURL, dir, initialBranch string, refs []string, trackRemote bool) (*git.Repository, error) {
	repo, err := git.PlainCloneContext(ctx, dir, false, &git.CloneOptions{URL: remoteURL, ReferenceName: plumbing.ReferenceName(initialBranch)})
	if err != nil {
		return nil, err
	}

	if len(refs) > 0 {
		err = Fetch(ctx, repo, DefaultRemoteName, refs, trackRemote)
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}

func CloneAndFetchToMemory(ctx context.Context, remoteURL, initialBranch string, refs []string, trackRemote bool) (*git.Repository, error) {
	repo, err := git.CloneContext(ctx, memory.NewStorage(), memfs.New(), &git.CloneOptions{URL: remoteURL, ReferenceName: plumbing.ReferenceName(initialBranch)})
	if err != nil {
		return nil, err
	}

	if len(refs) > 0 {
		err = Fetch(ctx, repo, DefaultRemoteName, refs, trackRemote)
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}
