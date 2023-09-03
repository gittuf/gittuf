package gitinterface

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
)

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

	return remote.Push(pushOpts)
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

	return remote.Fetch(fetchOpts)
}
