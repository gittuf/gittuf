package common

import (
	"bytes"
	"os"
	"os/exec"
	"strings"

	"github.com/go-git/go-git/v5"
)

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
