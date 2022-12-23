package gittuf

// All Git-specific utilities minus the gittuf store go here.

import (
	"bytes"
	"fmt"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	tufdata "github.com/theupdateframework/go-tuf/data"
)

func GetRepoRootDir() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.Trim(stdout.String(), "\n"), nil
}

func GetRepoHandler() (*git.Repository, error) {
	repoRoot, err := GetRepoRootDir()
	if err != nil {
		return &git.Repository{}, err
	}
	return git.PlainOpen(repoRoot)
}

func GetRefNameForHEAD() (string, error) {
	mainRepo, err := GetRepoHandler()
	if err != nil {
		return "", err
	}

	headRef, err := mainRepo.Storer.Reference(plumbing.HEAD)
	if err != nil {
		return "", err
	}
	branchSplit := strings.Split(headRef.Target().String(), "/")
	return branchSplit[len(branchSplit)-1], nil
}

func GetTipCommitIDForRef(refName string, refType int) (tufdata.HexBytes, error) {
	mainRepo, err := GetRepoHandler()
	if err != nil {
		return tufdata.HexBytes{}, err
	}

	var r plumbing.ReferenceName
	switch refType {
	case GitBranchRef:
		r = plumbing.NewBranchReferenceName(refName)
	case GitTagRef:
		r = plumbing.NewTagReferenceName(refName)
	default:
		return tufdata.HexBytes{}, fmt.Errorf("unknown reference type for %s", refName)
	}

	ref, err := mainRepo.Reference(r, false)
	if err != nil {
		return tufdata.HexBytes{}, err
	}

	return convertPlumbingHashToTUFHashHexBytes(ref.Hash()), nil
}

func GetHEADCommitID() (tufdata.HexBytes, error) {
	mainRepo, err := GetRepoHandler()
	if err != nil {
		return tufdata.HexBytes{}, err
	}
	headRef, err := mainRepo.Head()
	if err != nil {
		return tufdata.HexBytes{}, err
	}
	return convertPlumbingHashToTUFHashHexBytes(headRef.Hash()), nil
}

func UndoLastCommit(cause error) error {
	mainRepo, err := GetRepoHandler()
	if err != nil {
		return fmt.Errorf("could not undo commit triggered due to error %w", cause)
	}

	headRef, err := mainRepo.Head()
	if err != nil {
		return fmt.Errorf("could not undo commit triggered due to error %w", cause)
	}

	lastCommit, err := mainRepo.CommitObject(headRef.Hash())
	if err != nil {
		return fmt.Errorf("could not undo commit triggered due to error %w", cause)
	}

	if len(lastCommit.ParentHashes) == 0 {
		// This is the first commit
		refName, err := GetRefNameForHEAD()
		if err != nil {
			return fmt.Errorf("could not undo commit triggered due to error %w", cause)
		}
		err = mainRepo.Storer.RemoveReference(plumbing.NewBranchReferenceName(refName))
		if err != nil {
			return fmt.Errorf("could not undo commit triggered due to error %w", cause)
		}
		return cause
	} else if len(lastCommit.ParentHashes) > 1 {
		// This is a merge commit
		return fmt.Errorf("could not undo commit as it is a merge commit, triggered due to error %w", cause)
	}

	currentWorktree, err := mainRepo.Worktree()
	if err != nil {
		return fmt.Errorf("could not undo commit triggered due to error %w", cause)
	}

	err = currentWorktree.Reset(&git.ResetOptions{
		Commit: lastCommit.ParentHashes[0],
		Mode:   git.SoftReset,
	})
	if err != nil {
		return fmt.Errorf("could not undo commit triggered due to error %w", cause)
	}

	return cause
}

func convertPlumbingHashToTUFHashHexBytes(hash plumbing.Hash) tufdata.HexBytes {
	hb := make(tufdata.HexBytes, len(hash))
	for i := range hash {
		hb[i] = hash[i]
	}
	return hb
}

func convertTUFHashHexBytesToPlumbingHash(hb tufdata.HexBytes) plumbing.Hash {
	hash := plumbing.Hash{}
	for i := range hash {
		hash[i] = hb[i]
	}
	return hash
}
