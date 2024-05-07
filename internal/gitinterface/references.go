// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"fmt"
	"strings"
)

const (
	RefPrefix       = "refs/"
	BranchRefPrefix = "refs/heads/"
	TagRefPrefix    = "refs/tags/"
	RemoteRefPrefix = "refs/remotes/"
)

var (
	ErrReferenceNotFound = errors.New("requested Git reference not found")
)

func (r *Repository) SetReference(refName string, gitID Hash) error {
	_, stdErr, err := r.executeGitCommand("update-ref", "--create-reflog", refName, gitID.String())
	if err != nil {
		return fmt.Errorf("unable to set Git reference '%s' to '%s': %s", refName, gitID.String(), stdErr)
	}

	return nil
}

func (r *Repository) CheckAndSetReference(refName string, newGitID, oldGitID Hash) error {
	_, stdErr, err := r.executeGitCommand("update-ref", "--create-reflog", refName, newGitID.String(), oldGitID.String())
	if err != nil {
		return fmt.Errorf("unable to set Git reference '%s' to '%s': %s", refName, newGitID.String(), stdErr)
	}

	return nil
}

func (r *Repository) GetReference(refName string) (Hash, error) {
	stdOut, stdErr, err := r.executeGitCommand("rev-parse", refName)
	if err != nil {
		if strings.Contains(stdErr, "unknown revision or path not in the working tree") {
			return ZeroHash, ErrReferenceNotFound
		}
		return ZeroHash, fmt.Errorf("unable to read reference '%s': %s", refName, stdErr)
	}

	hash, err := NewHash(strings.TrimSpace(stdOut))
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid Git ID for reference '%s': %w", refName, err)
	}

	return hash, nil
}

func (r *Repository) GetSymbolicReferenceTarget(refName string) (string, error) {
	stdOut, stdErr, err := r.executeGitCommand("symbolic-ref", refName)
	if err != nil {
		return "", fmt.Errorf("unable to resolve %s: %s", refName, stdErr)
	}

	return strings.TrimSpace(stdOut), nil
}

func (r *Repository) AbsoluteReference(target string) (string, error) {
	if strings.HasPrefix(target, RefPrefix) {
		return target, nil
	}

	if target == "HEAD" {
		return r.GetSymbolicReferenceTarget("HEAD")
	}

	// Check if branch
	branchName := BranchReferenceName(target)
	_, err := r.GetReference(branchName)
	if err == nil {
		return branchName, nil
	}
	if !errors.Is(err, ErrReferenceNotFound) {
		return "", err
	}

	// Check if tag
	tagName := TagReferenceName(target)
	_, err = r.GetReference(tagName)
	if err == nil {
		return tagName, nil
	}
	if !errors.Is(err, ErrReferenceNotFound) {
		return "", err
	}

	return "", ErrReferenceNotFound
}

func (r *Repository) RefSpec(refName, remoteName string, fastForwardOnly bool) (string, error) {
	var (
		refPath string
		err     error
	)

	refPath = refName
	if !strings.HasPrefix(refPath, RefPrefix) {
		refPath, err = r.AbsoluteReference(refName)
		if err != nil {
			return "", err
		}
	}

	if strings.HasPrefix(refPath, TagRefPrefix) {
		// TODO: check if this is correct, AFAICT tags aren't tracked in the
		// remotes namespace.
		fastForwardOnly = true
	}

	// local is always refPath, destination depends on remoteName
	localPath := refPath
	var remotePath string
	if len(remoteName) > 0 {
		remotePath = RemoteRef(refPath, remoteName)
	} else {
		remotePath = refPath
	}

	refSpecString := fmt.Sprintf("%s:%s", localPath, remotePath)
	if !fastForwardOnly {
		refSpecString = fmt.Sprintf("+%s", refSpecString)
	}

	return refSpecString, nil
}

func BranchReferenceName(branchName string) string {
	if strings.HasPrefix(branchName, BranchRefPrefix) {
		return branchName
	}

	return fmt.Sprintf("%s%s", BranchRefPrefix, branchName)
}

func TagReferenceName(tagName string) string {
	if strings.HasPrefix(tagName, TagRefPrefix) {
		return tagName
	}

	return fmt.Sprintf("%s%s", TagRefPrefix, tagName)
}
