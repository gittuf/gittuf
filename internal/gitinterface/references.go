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

// GetReference returns the tip of the specified Git reference.
func (r *Repository) GetReference(refName string) (Hash, error) {
	refTipID, err := r.executeGitCommandString("rev-parse", refName)
	if err != nil {
		if strings.Contains(err.Error(), "unknown revision or path not in the working tree") {
			return ZeroHash, ErrReferenceNotFound
		}
		return ZeroHash, fmt.Errorf("unable to read reference '%s': %w", refName, err)
	}

	hash, err := NewHash(refTipID)
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid Git ID for reference '%s': %w", refName, err)
	}

	return hash, nil
}

// SetReference sets the specified reference to the provided Git ID.
func (r *Repository) SetReference(refName string, gitID Hash) error {
	_, stdErr, err := r.executeGitCommand("update-ref", "--create-reflog", refName, gitID.String())
	if err != nil {
		return fmt.Errorf("unable to set Git reference '%s' to '%s': %s", refName, gitID.String(), stdErr)
	}

	return nil
}

// CheckAndSetReference sets the specified reference to the provided Git ID if
// the reference is currently set to `oldGitID`.
func (r *Repository) CheckAndSetReference(refName string, newGitID, oldGitID Hash) error {
	_, stdErr, err := r.executeGitCommand("update-ref", "--create-reflog", refName, newGitID.String(), oldGitID.String())
	if err != nil {
		return fmt.Errorf("unable to set Git reference '%s' to '%s': %s", refName, newGitID.String(), stdErr)
	}

	return nil
}

// GetSymbolicReferenceTarget returns the name of the Git reference the provided
// symbolic Git reference is pointing to.
func (r *Repository) GetSymbolicReferenceTarget(refName string) (string, error) {
	symTarget, err := r.executeGitCommandString("symbolic-ref", refName)
	if err != nil {
		return "", fmt.Errorf("unable to resolve %s: %w", refName, err)
	}

	return symTarget, nil
}

// AbsoluteReference returns the fully qualified reference path for the provided
// Git ref.
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

// RefSpec creates a Git refspec for the specified ref.  For more information on
// the Git refspec, please consult:
// https://git-scm.com/book/en/v2/Git-Internals-The-Refspec.
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

// BranchReferenceName returns the full reference name for the specified branch
// in the form `refs/heads/<branchName>`.
func BranchReferenceName(branchName string) string {
	if strings.HasPrefix(branchName, BranchRefPrefix) {
		return branchName
	}

	return fmt.Sprintf("%s%s", BranchRefPrefix, branchName)
}

// TagReferenceName returns the full reference name for the specified tag in the
// form `refs/tags/<tagName>`.
func TagReferenceName(tagName string) string {
	if strings.HasPrefix(tagName, TagRefPrefix) {
		return tagName
	}

	return fmt.Sprintf("%s%s", TagRefPrefix, tagName)
}
