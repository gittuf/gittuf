package common

import (
	"os"
	"os/exec"

	"github.com/go-git/go-git/v5"
)

func GetRepositoryHandler() (*git.Repository, error) {
	return git.PlainOpenWithOptions(".", &git.PlainOpenOptions{DetectDotGit: true})
}

func CreateTestRepository() (string, error) {
	testDir, err := os.MkdirTemp("", "gittuf")
	if err != nil {
		return testDir, err
	}

	cmd := exec.Command("git", "init", testDir)
	return testDir, cmd.Run()
}
