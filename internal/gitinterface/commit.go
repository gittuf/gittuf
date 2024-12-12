// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
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
	gitConfig, err := r.GetGitConfig()
	if err != nil {
		return ZeroHash, err
	}

	commitMetadata := object.Signature{
		Name:  gitConfig["user.name"],
		Email: gitConfig["user.email"],
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
	commit.PGPSignature = signature

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

// verifyCommitSignature verifies a signature for the specified commit using
// the provided public key.
func (r *Repository) verifyCommitSignature(ctx context.Context, commitID Hash, key *signerverifier.SSLibKey) error {
	goGitRepo, err := r.GetGoGitRepository()
	if err != nil {
		return fmt.Errorf("error opening repository: %w", err)
	}

	commit, err := goGitRepo.CommitObject(plumbing.NewHash(commitID.String()))
	if err != nil {
		return fmt.Errorf("unable to load commit object: %w", err)
	}

	switch key.KeyType {
	case gpg.KeyType:
		if _, err := commit.Verify(key.KeyVal.Public); err != nil {
			return ErrIncorrectVerificationKey
		}

		return nil
	case ssh.KeyType:
		commitContents, err := getCommitBytesWithoutSignature(commit)
		if err != nil {
			return errors.Join(ErrVerifyingSSHSignature, err)
		}
		commitSignature := []byte(commit.PGPSignature)

		if err := verifySSHKeySignature(ctx, key, commitContents, commitSignature); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	case sigstore.KeyType:
		commitContents, err := getCommitBytesWithoutSignature(commit)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		commitSignature := []byte(commit.PGPSignature)

		if err := verifyGitsignSignature(ctx, r, key, commitContents, commitSignature); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	}

	return ErrUnknownSigningMethod
}

// GetCommitMessage returns the commit's message.
func (r *Repository) GetCommitMessage(commitID Hash) (string, error) {
	if err := r.ensureIsCommit(commitID); err != nil {
		return "", err
	}

	commitMessage, err := r.executor("show", "-s", "--format=%B", commitID.String()).executeString()
	if err != nil {
		return "", fmt.Errorf("unable to identify message for commit '%s': %w", commitID.String(), err)
	}

	return commitMessage, nil
}

// GetCommitTreeID returns the commit's Git tree ID.
func (r *Repository) GetCommitTreeID(commitID Hash) (Hash, error) {
	if err := r.ensureIsCommit(commitID); err != nil {
		return ZeroHash, err
	}

	stdOut, err := r.executor("rev-parse", fmt.Sprintf("%s^{tree}", commitID.String())).executeString()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to identify tree for commit '%s': %w", commitID.String(), err)
	}

	hash, err := NewHash(stdOut)
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid tree for commit ID '%s': %w", commitID, err)
	}
	return hash, nil
}

// GetCommitParentIDs returns the commit's parent commit IDs.
func (r *Repository) GetCommitParentIDs(commitID Hash) ([]Hash, error) {
	if err := r.ensureIsCommit(commitID); err != nil {
		return nil, err
	}

	stdOut, err := r.executor("rev-parse", fmt.Sprintf("%s^@", commitID.String())).executeString()
	if err != nil {
		return nil, fmt.Errorf("unable to identify parents for commit '%s': %w", commitID.String(), err)
	}

	commitIDSplit := strings.Split(stdOut, "\n")
	if len(commitIDSplit) == 0 {
		return nil, nil
	}

	commitIDs := []Hash{}
	for _, commitID := range commitIDSplit {
		if commitID == "" {
			continue
		}

		hash, err := NewHash(commitID)
		if err != nil {
			return nil, fmt.Errorf("invalid parent commit ID '%s': %w", commitID, err)
		}

		commitIDs = append(commitIDs, hash)
	}

	if len(commitIDs) == 0 {
		return nil, nil
	}

	return commitIDs, nil
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
	return err == nil, nil
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
