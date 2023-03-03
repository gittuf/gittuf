package common

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/go-git/go-git/v5"
)

const GittufNamespace = "refs/gittuf"

// InitializeGittufNamespace creates the refs/gittuf directory if it does not
// already exist. It does not initialize the refs for policy and RSL namespaces.
func InitializeGittufNamespace() error {
	repoRootDir, err := GetRepositoryRootDirectory()
	if err != nil {
		return err
	}

	refPath := filepath.Join(repoRootDir, GetGitDir(), GittufNamespace)
	if _, err := os.Stat(refPath); err != nil {
		if os.IsNotExist(err) {
			if err := os.Mkdir(refPath, 0755); err != nil {
				return err
			}
		} else {
			return err
		}
	}

	return nil
}

func GetRepositoryRootDirectory() (string, error) {
	cmd := exec.Command("git", "rev-parse", "--show-toplevel")
	var stdout bytes.Buffer
	cmd.Stdout = &stdout
	err := cmd.Run()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(stdout.String()), nil
}

func GetRepositoryHandler() (*git.Repository, error) {
	repoRoot, err := GetRepositoryRootDirectory()
	if err != nil {
		return &git.Repository{}, err
	}
	return git.PlainOpen(repoRoot)
}

func CreateTestRepository() (string, error) {
	testDir, err := os.MkdirTemp("", "gittuf")
	if err != nil {
		return testDir, err
	}
	cmd := exec.Command("git", "init", testDir)
	return testDir, cmd.Run()
}

func GetGitDir() string {
	// TODO: handle detached gitdir
	return ".git"
}
