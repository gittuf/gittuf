package gitinterface

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func GetTip(repo *git.Repository, refName string) (plumbing.Hash, error) {
	ref, err := repo.Reference(plumbing.ReferenceName(refName), true)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return ref.Hash(), nil
}

func ResetCommit(repo *git.Repository, refName string, commitID plumbing.Hash) error {
	currentHEAD, err := repo.Head()
	if err != nil {
		return err
	}

	wt, err := repo.Worktree()
	if err != nil {
		return err
	}

	if err := wt.Checkout(&git.CheckoutOptions{Branch: plumbing.ReferenceName(refName)}); err != nil {
		return err
	}

	if err := wt.Reset(&git.ResetOptions{Commit: commitID, Mode: git.MergeReset}); err != nil {
		return err
	}

	if currentHEAD.Type() == plumbing.HashReference {
		return wt.Checkout(&git.CheckoutOptions{Hash: currentHEAD.Hash()})
	}

	return wt.Checkout(&git.CheckoutOptions{Branch: currentHEAD.Name()})
}

func ResetDueToError(cause error, repo *git.Repository, refName string, commitID plumbing.Hash) error {
	if err := ResetCommit(repo, refName, commitID); err != nil {
		return fmt.Errorf("unable to reset %s to %s, caused by following error: %w", refName, commitID.String(), cause)
	}
	return cause
}
