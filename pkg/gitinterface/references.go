// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"fmt"
	"os"
	"path"
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
	refTipID, err := r.executor("rev-parse", refName).executeString()
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
	_, err := r.executor("update-ref", "--create-reflog", refName, gitID.String()).executeString()
	if err != nil {
		return fmt.Errorf("unable to set Git reference '%s' to '%s': %w", refName, gitID.String(), err)
	}

	return nil
}

// DeleteReference deletes the specified Git reference.
func (r *Repository) DeleteReference(refName string) error {
	_, err := r.executor("update-ref", "-d", refName).executeString()
	if err != nil {
		return fmt.Errorf("unable to delete Git reference '%s': %w", refName, err)
	}
	return nil
}

// CheckAndSetReference sets the specified reference to the provided Git ID if
// the reference is currently set to `oldGitID`.
func (r *Repository) CheckAndSetReference(refName string, newGitID, oldGitID Hash) error {
	_, err := r.executor("update-ref", "--create-reflog", refName, newGitID.String(), oldGitID.String()).executeString()
	if err != nil {
		return fmt.Errorf("unable to set Git reference '%s' to '%s': %w", refName, newGitID.String(), err)
	}

	return nil
}

// GetSymbolicReferenceTarget returns the name of the Git reference the provided
// symbolic Git reference is pointing to.
func (r *Repository) GetSymbolicReferenceTarget(refName string) (string, error) {
	symTarget, err := r.executor("symbolic-ref", refName).executeString()
	if err != nil {
		return "", fmt.Errorf("unable to resolve %s: %w", refName, err)
	}

	return symTarget, nil
}

// SetSymbolicReference sets the specified symbolic reference to the specified
// target reference.
func (r *Repository) SetSymbolicReference(symRefName, targetRefName string) error {
	_, err := r.executor("symbolic-ref", symRefName, targetRefName).executeString()
	if err != nil {
		return fmt.Errorf("unable to set symbolic Git reference '%s' to '%s': %w", symRefName, targetRefName, err)
	}

	return nil
}

// AbsoluteReference returns the fully qualified reference path for the provided
// Git ref.
// Source: https://git-scm.com/docs/gitrevisions#Documentation/gitrevisions.txt-emltrefnamegtemegemmasterememheadsmasterememrefsheadsmasterem
func (r *Repository) AbsoluteReference(target string) (string, error) {
	_, err := os.Stat(path.Join(r.gitDirPath, target))
	if err == nil {
		if strings.HasPrefix(target, RefPrefix) {
			// not symbolic ref
			return target, nil
		}
		// symbolic ref such as .git/HEAD
		return r.GetSymbolicReferenceTarget(target)
	}

	// We may have a ref that isn't available locally but is still ref-prefixed.
	if strings.HasPrefix(target, RefPrefix) {
		return target, nil
	}

	// If target is a full ref already and it's stored in the GIT_DIR/refs
	// directory, we don't reach this point. Below, we handle cases where the
	// ref may be packed.

	// Check if custom reference
	customName := CustomReferenceName(target)
	_, err = r.GetReference(customName)
	if err == nil {
		return customName, nil
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

	// Check if branch
	branchName := BranchReferenceName(target)
	_, err = r.GetReference(branchName)
	if err == nil {
		return branchName, nil
	}
	if !errors.Is(err, ErrReferenceNotFound) {
		return "", err
	}

	// Check if remote tracker ref
	remoteRefName := RemoteReferenceName(target)
	_, err = r.GetReference(remoteRefName)
	if err == nil {
		return remoteRefName, nil
	}
	if !errors.Is(err, ErrReferenceNotFound) {
		return "", err
	}

	remoteRefHEAD := path.Join(remoteRefName, "HEAD")
	_, err = r.GetReference(remoteRefHEAD)
	if err == nil {
		return remoteRefHEAD, nil
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

// CustomReferenceName returns the full reference name in the form
// `refs/<customName>`.
func CustomReferenceName(customName string) string {
	if strings.HasPrefix(customName, RefPrefix) {
		return customName
	}

	return fmt.Sprintf("%s%s", RefPrefix, customName)
}

// TagReferenceName returns the full reference name for the specified tag in the
// form `refs/tags/<tagName>`.
func TagReferenceName(tagName string) string {
	if strings.HasPrefix(tagName, TagRefPrefix) {
		return tagName
	}

	return fmt.Sprintf("%s%s", TagRefPrefix, tagName)
}

// BranchReferenceName returns the full reference name for the specified branch
// in the form `refs/heads/<branchName>`.
func BranchReferenceName(branchName string) string {
	if strings.HasPrefix(branchName, BranchRefPrefix) {
		return branchName
	}

	return fmt.Sprintf("%s%s", BranchRefPrefix, branchName)
}

// RemoteReferenceName returns the full reference name in the form
// `refs/remotes/<name>`.
func RemoteReferenceName(name string) string {
	if strings.HasPrefix(name, RemoteRefPrefix) {
		return name
	}

	return fmt.Sprintf("%s%s", RemoteRefPrefix, name)
}
