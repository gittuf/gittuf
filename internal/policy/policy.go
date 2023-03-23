package policy

import (
	"os"
	"path/filepath"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/go-git/go-git/v5/plumbing"
)

const (
	PolicyRef        = "refs/gittuf/policy"
	PolicyStagingRef = "refs/gittuf/policy-staging"
)

// InitializeNamespace creates a git ref for the policy. Initially, the entry
// has a zero hash.
// Note: policy.InitializeNamespace assumes the gittuf namespace has been
// created already.
func InitializeNamespace() error {
	repoRootDir, err := common.GetRepositoryRootDirectory()
	if err != nil {
		return err
	}

	refPaths := []string{
		filepath.Join(repoRootDir, common.GetGitDir(), PolicyRef),
		filepath.Join(repoRootDir, common.GetGitDir(), PolicyStagingRef),
	}
	for _, refPath := range refPaths {
		if _, err := os.Stat(refPath); err != nil {
			if os.IsNotExist(err) {
				if err := os.WriteFile(refPath, plumbing.ZeroHash[:], 0644); err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}

	return nil
}
