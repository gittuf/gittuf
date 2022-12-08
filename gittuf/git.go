package gittuf

// All Git-specific utilities minus the gittuf store go here.

import (
	"bytes"
	"fmt"
	"net/url"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	tufdata "github.com/theupdateframework/go-tuf/data"
)

const (
	BranchRef = iota
	TagRef
)

func ParseGitTarget(uri string) (string, uint, error) {
	u, err := url.Parse(uri)
	if err != nil {
		return "", 0, err
	}
	if u.Scheme != "git" {
		return "", 0, fmt.Errorf("%s is not a Git object", uri)
	}

	if len(u.Opaque) == 0 {
		return "", 0, fmt.Errorf("invalid format for %s", uri)
	}

	split := strings.Split(u.Opaque, "=") // assuming there aren't multiple key value pairs
	if len(split) > 2 {
		return "", 0, fmt.Errorf("invalid format for %s", uri)
	}

	if split[0] == "tag" {
		return split[1], TagRef, nil
	}
	return split[1], BranchRef, nil
}

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

func GetTipCommitIDForRef(refName string, refType uint) (tufdata.HexBytes, error) {
	mainRepo, err := GetRepoHandler()
	if err != nil {
		return tufdata.HexBytes{}, err
	}

	var r plumbing.ReferenceName
	switch refType {
	case BranchRef:
		r = plumbing.NewBranchReferenceName(refName)
	case TagRef:
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
		return fmt.Errorf("%w, in turn triggered due to error %w", err, cause)
	}

	headRef, err := mainRepo.Head()
	if err != nil {
		return fmt.Errorf("%w, in turn triggered due to error %w", err, cause)
	}

	lastCommit, err := mainRepo.CommitObject(headRef.Hash())
	if err != nil {
		return fmt.Errorf("%w, in turn triggered due to error %w", err, cause)
	}

	if len(lastCommit.ParentHashes) == 0 {
		// This is the first commit
		refName, err := GetRefNameForHEAD()
		if err != nil {
			return fmt.Errorf("%w, in turn triggered due to error %w", err, cause)
		}
		err = mainRepo.Storer.RemoveReference(plumbing.NewBranchReferenceName(refName))
		if err != nil {
			return fmt.Errorf("%w, in turn triggered due to error %w", err, cause)
		}
		return cause
	} else if len(lastCommit.ParentHashes) > 1 {
		// This is a merge commit
		return fmt.Errorf("unable to undo last commit %s as it has multiple parent commits, in turn triggered due to error %w", lastCommit.Hash.String(), cause)
	}

	currentWorktree, err := mainRepo.Worktree()
	if err != nil {
		return fmt.Errorf("%w, in turn triggered due to error %w", err, cause)
	}

	err = currentWorktree.Reset(&git.ResetOptions{
		Commit: lastCommit.ParentHashes[0],
		Mode:   git.SoftReset,
	})
	if err != nil {
		return fmt.Errorf("%w, in turn triggered due to error %w", err, cause)
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
