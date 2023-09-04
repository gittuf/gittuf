package gitinterface

import (
	"errors"
	"fmt"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/transport"
	"github.com/go-git/go-git/v5/storage/memory"
)

const DefaultRemoteName = "origin"

func Push(repo *git.Repository, remoteName string, refs []config.RefSpec) error {
	remote, err := repo.Remote(remoteName)
	if err != nil {
		return err
	}

	pushOpts := &git.PushOptions{
		RemoteName: remoteName,
		RefSpecs:   refs,
		Atomic:     true,
		Force:      true,
	}

	err = remote.Push(pushOpts)
	if errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

func Fetch(repo *git.Repository, remoteName string, refs []config.RefSpec) error {
	remote, err := repo.Remote(remoteName)
	if err != nil {
		return err
	}

	fetchOpts := &git.FetchOptions{
		RemoteName: remoteName,
		RefSpecs:   refs,
	}

	err = remote.Fetch(fetchOpts)
	if errors.Is(err, transport.ErrEmptyRemoteRepository) || errors.Is(err, git.NoErrAlreadyUpToDate) {
		return nil
	}
	return err
}

func CloneAndFetch(remoteURL, dir, initialBranch string, refs []config.RefSpec) (*git.Repository, error) {
	repo, err := git.PlainClone(dir, false, &git.CloneOptions{URL: remoteURL, ReferenceName: plumbing.ReferenceName(initialBranch)})
	if err != nil {
		return nil, err
	}

	if len(refs) > 0 {
		err = Fetch(repo, DefaultRemoteName, refs)
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}

func CloneAndFetchToMemory(remoteURL, initialBranch string, refs []config.RefSpec) (*git.Repository, error) {
	repo, err := git.Clone(memory.NewStorage(), memfs.New(), &git.CloneOptions{URL: remoteURL, ReferenceName: plumbing.ReferenceName(initialBranch)})
	if err != nil {
		fmt.Println(err)
		return nil, err
	}

	if len(refs) > 0 {
		err = Fetch(repo, DefaultRemoteName, refs)
		if err != nil {
			return nil, err
		}
	}

	return repo, nil
}
