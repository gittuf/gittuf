package gitinterface

import (
	"fmt"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

// GetTip returns the hash of the tip of the specified ref.
func GetTip(repo *git.Repository, refName string) (plumbing.Hash, error) {
	ref, err := repo.Reference(plumbing.ReferenceName(refName), true)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	return ref.Hash(), nil
}

// ResetCommit sets a Git reference with the name refName to the commit
// specified by its hash as commitID. Note that the commit must already be in
// the repository's object store.
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

// ResetDueToError is a helper used to reverse a change applied to a ref due to
// an error encountered after the change but part of the same operation. This
// ensures that gittuf operations are atomic. Otherwise, a repository may enter
// a violation state where a ref is updated without accompanying RSL entries or
// other metadata changes.
func ResetDueToError(cause error, repo *git.Repository, refName string, commitID plumbing.Hash) error {
	if err := ResetCommit(repo, refName, commitID); err != nil {
		return fmt.Errorf("unable to reset %s to %s, caused by following error: %w", refName, commitID.String(), cause)
	}
	return cause
}
