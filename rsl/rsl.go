package rsl

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/adityasaky/gittuf/internal/gitinterface"
	"github.com/go-git/go-git/v5/plumbing"
)

const RSLRef = "refs/gittuf/reference-state-log"

// InitializeNamespace creates a git ref for the reference state log. Initially,
// the entry has a zero hash.
// Note: rsl.InitializeNamespace assumes the gittuf namespace has been created
// already.
func InitializeNamespace() error {
	repoRootDir, err := common.GetRepositoryRootDirectory()
	if err != nil {
		return err
	}

	refPath := filepath.Join(repoRootDir, common.GetGitDir(), RSLRef)
	if _, err := os.Stat(refPath); err != nil {
		if os.IsNotExist(err) {
			if err := os.WriteFile(refPath, plumbing.ZeroHash[:], 0644); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

// AddEntry adds an RSL entry to the log for the ref and hash passed in.
func AddEntry(refName string, commitID plumbing.Hash, sign bool) error {
	repo, err := common.GetRepositoryHandler()
	if err != nil {
		return err
	}
	message := fmt.Sprintf("%s: %s", refName, commitID.String())

	return gitinterface.Commit(repo, plumbing.ZeroHash, RSLRef, message, sign)
}

// GetLatestEntry returns the latest entry available locally in the RSL.
// TODO: There is no information yet about the signature for the entry.
func GetLatestEntry() (string, plumbing.Hash, error) {
	repo, err := common.GetRepositoryHandler()
	if err != nil {
		return "", plumbing.ZeroHash, err
	}

	ref, err := repo.Reference(plumbing.ReferenceName(RSLRef), true)
	if err != nil {
		return "", plumbing.ZeroHash, err
	}

	commitObj, err := repo.CommitObject(ref.Hash())
	if err != nil {
		return "", plumbing.ZeroHash, err
	}

	messageSplit := strings.Split(commitObj.Message, ":")

	return strings.Trim(messageSplit[0], " "),
		plumbing.NewHash(strings.Trim(messageSplit[1], " ")), nil
}
