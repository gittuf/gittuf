package gittuf

import (
	"errors"
	"fmt"

	"github.com/adityasaky/gittuf/internal/gitstore"
	"github.com/go-git/go-git/v5"
	gitconfig "github.com/go-git/go-git/v5/config"
)

func Push(store *gitstore.GitStore, remoteName string, refName string) error {
	if err := store.State().PushToRemote(remoteName); err != nil {
		return err
	}

	if err := store.Repository().Push(&git.PushOptions{
		RemoteName: remoteName,
		RefSpecs: []gitconfig.RefSpec{
			gitconfig.RefSpec(fmt.Sprintf("refs/heads/%s:refs/heads/%s", refName, refName)),
		},
	}); err != nil && !errors.Is(err, git.NoErrAlreadyUpToDate) {
		return err
	}

	return nil
}
