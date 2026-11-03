// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/storage/memory"
)

// Commit creates a new commit in the repo and sets targetRef's to the commit.
// This function is meant only for gittuf references, and therefore it does not
// mutate repository worktrees.
func (r *Repository) Commit(treeID Hash, targetRef, message string, sign bool) (Hash, error) {
	currentGitID, err := r.GetReference(targetRef)
	if err != nil {
		if !errors.Is(err, ErrReferenceNotFound) {
			return ZeroHash, err
		}
	}

	args := []string{"commit-tree", "-m", message}

	if !currentGitID.IsZero() {
		args = append(args, "-p", currentGitID.String())
	}

	if sign {
		args = append(args, "-S")
	}

	args = append(args, treeID.String())

	now := r.clock.Now().Format(time.RFC3339)
	env := []string{fmt.Sprintf("%s=%s", committerTimeKey, now), fmt.Sprintf("%s=%s", authorTimeKey, now)}

	stdOut, err := r.executor(args...).withEnv(env...).executeString()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to create commit: %w", err)
	}
	commitID, err := NewHash(stdOut)
	if err != nil {
		return ZeroHash, fmt.Errorf("received invalid commit ID: %w", err)
	}

	return commitID, r.CheckAndSetReference(targetRef, commitID, currentGitID)
}

// CommitUsingSpecificKey creates a new commit in the repository for the
// specified parameters. The commit is signed using the PEM encoded SSH or GPG
// private key. This function is expected for use in tests and gittuf's
// developer mode. In standard workflows, Commit() must be used instead which
// infers the signing key from the user's Git config.
func (r *Repository) CommitUsingSpecificKey(treeID Hash, targetRef, message string, signingKeyPEMBytes []byte) (Hash, error) {
	name, _, err := r.LookupConfig(gitstore.ConfigUserName)
	if err != nil {
		return ZeroHash, err
	}
	email, _, err := r.LookupConfig(gitstore.ConfigUserEmail)
	if err != nil {
		return ZeroHash, err
	}

	commitMetadata := object.Signature{
		Name:  name,
		Email: email,
		When:  r.clock.Now(),
	}

	commit := &object.Commit{
		Author:    commitMetadata,
		Committer: commitMetadata,
		TreeHash:  plumbing.NewHash(treeID.String()),
		Message:   message,
	}

	refTip, err := r.GetReference(targetRef)
	if err != nil {
		if !errors.Is(err, ErrReferenceNotFound) {
			return ZeroHash, err
		}
	}

	if !refTip.IsZero() {
		commit.ParentHashes = []plumbing.Hash{plumbing.NewHash(refTip.String())}
	}

	commitContents, err := getCommitBytesWithoutSignature(commit)
	if err != nil {
		return ZeroHash, err
	}
	signature, err := signGitObjectUsingKey(commitContents, signingKeyPEMBytes)
	if err != nil {
		return ZeroHash, err
	}
	// SHA-256 repositories store the signature under the `gpgsig-sha256`
	// header (go-git's SignatureSHA256), matching Git's own behavior, so it
	// can be read back during verification.
	if r.GetObjectFormat() == ObjectFormatSHA256 {
		commit.SignatureSHA256 = signature
	} else {
		commit.Signature = signature
	}

	goGitRepo, err := r.GetGoGitRepository()
	if err != nil {
		return ZeroHash, err
	}

	obj := goGitRepo.Storer.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return ZeroHash, err
	}
	commitID, err := goGitRepo.Storer.SetEncodedObject(obj)
	if err != nil {
		return ZeroHash, err
	}

	commitIDHash, err := NewHash(commitID.String())
	if err != nil {
		return ZeroHash, err
	}

	return commitIDHash, r.CheckAndSetReference(targetRef, commitIDHash, refTip)
}

// commitWithParents creates a new commit in the repo but does not update any
// references. It is only meant to be used for tests, and therefore accepts
// specific parent commit IDs.
func (r *Repository) commitWithParents(t *testing.T, treeID Hash, parentIDs []Hash, message string, sign bool) Hash { //nolint:unparam
	args := []string{"commit-tree", "-m", message}

	for _, commitID := range parentIDs {
		args = append(args, "-p", commitID.String())
	}

	if sign {
		args = append(args, "-S")
	}

	args = append(args, treeID.String())

	now := r.clock.Now().Format(time.RFC3339)
	env := []string{fmt.Sprintf("%s=%s", committerTimeKey, now), fmt.Sprintf("%s=%s", authorTimeKey, now)}

	stdOut, err := r.executor(args...).withEnv(env...).executeString()
	if err != nil {
		t.Fatal(fmt.Errorf("unable to create commit: %w", err))
	}
	commitID, err := NewHash(stdOut)
	if err != nil {
		t.Fatal(fmt.Errorf("received invalid commit ID: %w", err))
	}

	return commitID
}

// GetCommitInfo returns the commit's tree ID, parent IDs, and message using a
// single Git invocation. It returns an error if the requested ID is not a
// commit.
func (r *Repository) GetCommitInfo(commitID githash.Hash) (gitstore.CommitInfo, error) {
	stdOut, err := r.executor("rev-list", "--no-walk", "--no-commit-header", "--format=%H%x00%T%x00%P%x00%B", commitID.String()).executeString()
	if err != nil {
		return gitstore.CommitInfo{}, fmt.Errorf("unable to read commit '%s': %w", commitID.String(), err)
	}

	// rev-list does not reject non-commits: it prints nothing for blobs and
	// trees, and peels annotated tags to the commit they point to. The checks
	// below reject both cases.
	fields := strings.Split(stdOut, "\x00")
	if len(fields) != 4 {
		return gitstore.CommitInfo{}, fmt.Errorf("unexpected output reading '%s', is it a commit?", commitID.String())
	}
	if fields[0] != commitID.String() {
		return gitstore.CommitInfo{}, fmt.Errorf("requested Git ID '%s' is not a commit object", commitID.String())
	}

	treeID, err := githash.NewHash(fields[1])
	if err != nil {
		return gitstore.CommitInfo{}, fmt.Errorf("invalid tree ID '%s' for commit '%s': %w", fields[1], commitID.String(), err)
	}

	var parentIDs []githash.Hash
	for _, parent := range strings.Fields(fields[2]) {
		parentID, err := githash.NewHash(parent)
		if err != nil {
			return gitstore.CommitInfo{}, fmt.Errorf("invalid parent commit ID '%s': %w", parent, err)
		}
		parentIDs = append(parentIDs, parentID)
	}

	return gitstore.CommitInfo{
		TreeID:    treeID,
		ParentIDs: parentIDs,
		Message:   strings.TrimSpace(fields[3]),
	}, nil
}

// KnowsCommit returns true if the `testCommit` is a descendent of the
// `ancestorCommit`. That is, the testCommit _knows_ the ancestorCommit as it
// has a path in the commit graph to the ancestorCommit.
func (r *Repository) KnowsCommit(testCommitID, ancestorCommitID Hash) (bool, error) {
	if err := r.ensureIsCommit(testCommitID); err != nil {
		return false, err
	}
	if err := r.ensureIsCommit(ancestorCommitID); err != nil {
		return false, err
	}

	_, err := r.executor("merge-base", "--is-ancestor", ancestorCommitID.String(), testCommitID.String()).executeString()
	if err == nil {
		return true, nil
	}

	if exitErr, ok := errors.AsType[*exec.ExitError](err); ok && exitErr.ExitCode() == 1 {
		return false, nil
	}

	return false, err
}

// GetCommonAncestor finds the common ancestor commit for the two supplied
// commits.
func (r *Repository) GetCommonAncestor(commitAID, commitBID Hash) (Hash, error) {
	if err := r.ensureIsCommit(commitAID); err != nil {
		return nil, err
	}
	if err := r.ensureIsCommit(commitBID); err != nil {
		return nil, err
	}

	mergeBase, err := r.executor("merge-base", commitAID.String(), commitBID.String()).executeString()
	if err != nil {
		return nil, err
	}

	mergeBaseID, err := NewHash(mergeBase)
	if err != nil {
		return nil, fmt.Errorf("received invalid commit ID: %w", err)
	}
	return mergeBaseID, nil
}

// ensureIsCommit is a helper to check that the ID represents a Git commit
// object.
func (r *Repository) ensureIsCommit(commitID Hash) error {
	objType, err := r.executor("cat-file", "-t", commitID.String()).executeString()
	if err != nil {
		return fmt.Errorf("unable to inspect if object is commit: %w", err)
	} else if objType != "commit" {
		return fmt.Errorf("requested Git ID '%s' is not a commit object", commitID.String())
	}

	return nil
}

func getCommitBytesWithoutSignature(commit *object.Commit) ([]byte, error) {
	commitEncoded := memory.NewStorage().NewEncodedObject()
	if err := commit.EncodeWithoutSignature(commitEncoded); err != nil {
		return nil, err
	}
	r, err := commitEncoded.Reader()
	if err != nil {
		return nil, err
	}

	return io.ReadAll(r)
}
