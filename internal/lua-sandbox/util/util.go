// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// This file contains modified code from the lua-sandbox project, available at
// https://github.com/kikito/lua-sandbox/blob/master/sandbox.lua, and licensed
// under the MIT License

package sandbox

import (
	"os/exec"
	"path/filepath"
	"strings"
)

var AllowedDir = getGitRoot()

// GetGitRoot returns the root directory of the git repository
func getGitRoot() string {
	// Check if the current directory is a git repository
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	output, err := cmd.CombinedOutput()
	if err != nil || strings.TrimSpace(string(output)) != "true" {
		// Get the path to the .git directory
		cmd = exec.Command("git", "rev-parse", "--git-dir")
		output, err = cmd.CombinedOutput()
		if err != nil {
			return ""
		}
		gitDir := strings.TrimSpace(string(output))
		if gitDir == "." || gitDir == ".git" {
			// If the .git directory is the current directory
			// then the root directory is the parent directory
			cmd = exec.Command("git", "rev-parse", "--show-cdup")
			output, err = cmd.CombinedOutput()
			if err != nil {
				return ""
			}
			relativeRootDir := strings.TrimSpace(string(output))
			if relativeRootDir == "" {
				relativeRootDir = "."
			}
			absoluteRootDir, err := filepath.Abs(relativeRootDir)
			if err != nil {
				return ""
			}
			return absoluteRootDir
		}
		absoluteGitDir, err := filepath.Abs(gitDir)
		if err != nil {
			return ""
		}
		return absoluteGitDir
	}

	// Get the root directory of the git repository if the current directory
	// is already inside the working tree
	cmd = exec.Command("git", "rev-parse", "--show-toplevel")
	output, err = cmd.CombinedOutput()
	if err != nil {
		return ""
	}
	rootDir := strings.TrimSpace(string(output))
	absoluteRootDir, err := filepath.Abs(rootDir)
	if err != nil {
		return ""
	}
	return absoluteRootDir
}

// Check if the path is allowed to access
func IsPathAllowed(path string) bool {
	absPath, err := filepath.Abs(path)
	if err != nil {
		return false
	}
	absAllowedDir, _ := filepath.Abs(AllowedDir)
	return strings.HasPrefix(absPath, absAllowedDir)
}

// GetGitDiffOutput returns the output of the git diff command
func GetGitDiffOutput() (string, error) {
	cmd := exec.Command("git", "diff", "HEAD", "--no-ext-diff", "--unified=0", "-a", "--no-prefix")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}

// GetGitDiffFiles returns the list of files that are staged for commit
func GetGitDiffFiles() ([]string, error) {
	cmd := exec.Command("git", "diff", "--staged", "--name-only", "--diff-filter=A")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

// GetWorkTreeFiles returns the list of files in the working tree
func GetWorkTreeFiles() ([]string, error) {
	cmd := exec.Command("git", "ls-files")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return nil, err
	}
	return strings.Split(strings.TrimSpace(string(output)), "\n"), nil
}

// GetCurrentBranchName returns the name of the current branch
func GetCurrentBranchName() (string, error) {
	cmd := exec.Command("git", "symbolic-ref", "--short", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(output)), nil
}

// GetGitArchive returns the output of the git archive command
func GetGitArchive() (string, error) {
	cmd := exec.Command("git", "archive", "--format=tar", "HEAD")
	output, err := cmd.CombinedOutput()
	if err != nil {
		return "", err
	}
	return string(output), nil
}
