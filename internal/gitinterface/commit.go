// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jonboulle/clockwork"
)

// Commit creates a new commit in the repo and sets targetRef's HEAD to the
// commit.
func Commit(repo *git.Repository, treeHash plumbing.Hash, targetRef string, message string, sign bool) (plumbing.Hash, error) {
	gitConfig, err := getGitConfig(repo)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	targetRefTyped := plumbing.ReferenceName(targetRef)
	curRef, err := repo.Reference(targetRefTyped, true)
	if err != nil {
		// FIXME: this is a bit messy
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			// Set empty ref
			if err := repo.Storer.SetReference(plumbing.NewHashReference(targetRefTyped, plumbing.ZeroHash)); err != nil {
				return plumbing.ZeroHash, err
			}
			curRef, err = repo.Reference(targetRefTyped, true)
			if err != nil {
				return plumbing.ZeroHash, err
			}
		} else {
			return plumbing.ZeroHash, err
		}
	}

	commit := CreateCommitObject(gitConfig, treeHash, []plumbing.Hash{curRef.Hash()}, message, clock)

	if sign {
		signature, err := signCommit(commit)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		commit.PGPSignature = signature
	}

	return ApplyCommit(repo, commit, curRef)
}

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

	stdOut, err := r.executeGitCommandWithEnvString(env, args...)
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
func CommitUsingSpecificKey(repo *git.Repository, treeHash plumbing.Hash, targetRef, message string, signingKeyPEMBytes []byte) (plumbing.Hash, error) {
	// Fetch gitConfig for author / committer information
	gitConfig, err := getGitConfig(repo)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	targetRefTyped := plumbing.ReferenceName(targetRef)
	curRef, err := repo.Reference(targetRefTyped, true)
	if err != nil {
		// FIXME: this is a bit messy
		if errors.Is(err, plumbing.ErrReferenceNotFound) {
			// Set empty ref
			if err := repo.Storer.SetReference(plumbing.NewHashReference(targetRefTyped, plumbing.ZeroHash)); err != nil {
				return plumbing.ZeroHash, err
			}
			curRef, err = repo.Reference(targetRefTyped, true)
			if err != nil {
				return plumbing.ZeroHash, err
			}
		} else {
			return plumbing.ZeroHash, err
		}
	}

	commit := CreateCommitObject(gitConfig, treeHash, []plumbing.Hash{curRef.Hash()}, message, clock)

	commitContents, err := getCommitBytesWithoutSignature(commit)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	signature, err := signGitObjectUsingKey(commitContents, signingKeyPEMBytes)
	if err != nil {
		return plumbing.ZeroHash, err
	}
	commit.PGPSignature = signature

	return ApplyCommit(repo, commit, curRef)
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

// ApplyCommit writes a commit object in the repository and updates the
// specified reference to point to the commit.
func ApplyCommit(repo *git.Repository, commit *object.Commit, curRef *plumbing.Reference) (plumbing.Hash, error) {
	commitHash, err := WriteCommit(repo, commit)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	newRef := plumbing.NewHashReference(curRef.Name(), commitHash)
	return commitHash, repo.Storer.CheckAndSetReference(newRef, curRef)
}

// WriteCommit stores the commit object in the repository's object store,
// returning the new commit's ID.
func WriteCommit(repo *git.Repository, commit *object.Commit) (plumbing.Hash, error) {
	obj := repo.Storer.NewEncodedObject()
	if err := commit.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}

	return repo.Storer.SetEncodedObject(obj)
}

// VerifyCommitSignature is used to verify a cryptographic signature associated
// with commit using TUF public keys.
func VerifyCommitSignature(ctx context.Context, commit *object.Commit, key *tuf.Key) error {
	switch key.KeyType {
	case signerverifier.GPGKeyType:
		if _, err := commit.Verify(key.KeyVal.Public); err != nil {
			return ErrIncorrectVerificationKey
		}

		return nil
	case signerverifier.RSAKeyType, signerverifier.ECDSAKeyType, signerverifier.ED25519KeyType, ssh.SSHKeyType:
		commitContents, err := getCommitBytesWithoutSignature(commit)
		if err != nil {
			return errors.Join(ErrVerifyingSSHSignature, err)
		}
		commitSignature := []byte(commit.PGPSignature)

		if err := verifySSHKeySignature(key, commitContents, commitSignature); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	case signerverifier.FulcioKeyType:
		commitContents, err := getCommitBytesWithoutSignature(commit)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		commitSignature := []byte(commit.PGPSignature)

		if err := verifyGitsignSignature(ctx, key, commitContents, commitSignature); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	}

	return ErrUnknownSigningMethod
}

// verifyCommitSignature verifies a signature for the specified commit using
// the provided public key.
func (r *Repository) verifyCommitSignature(ctx context.Context, commitID Hash, key *tuf.Key) error {
	goGitRepo, err := r.GetGoGitRepository()
	if err != nil {
		return fmt.Errorf("error opening repository: %w", err)
	}

	commit, err := goGitRepo.CommitObject(plumbing.NewHash(commitID.String()))
	if err != nil {
		return fmt.Errorf("unable to load commit object: %w", err)
	}

	switch key.KeyType {
	case signerverifier.GPGKeyType:
		if _, err := commit.Verify(key.KeyVal.Public); err != nil {
			return ErrIncorrectVerificationKey
		}

		return nil
	case signerverifier.RSAKeyType, signerverifier.ECDSAKeyType, signerverifier.ED25519KeyType, ssh.SSHKeyType:
		commitContents, err := getCommitBytesWithoutSignature(commit)
		if err != nil {
			return errors.Join(ErrVerifyingSSHSignature, err)
		}
		commitSignature := []byte(commit.PGPSignature)

		if err := verifySSHKeySignature(key, commitContents, commitSignature); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	case signerverifier.FulcioKeyType:
		commitContents, err := getCommitBytesWithoutSignature(commit)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		commitSignature := []byte(commit.PGPSignature)

		if err := verifyGitsignSignature(ctx, key, commitContents, commitSignature); err != nil {
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

	commitMessage, err := r.executeGitCommandString("show", "-s", "--format=%B", commitID.String())
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

	stdOut, err := r.executeGitCommandString("show", "-s", "--format=%T", commitID.String())
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

	stdOut, err := r.executeGitCommandString("show", "-s", "--format=%P", commitID.String())
	if err != nil {
		return nil, fmt.Errorf("unable to identify parents for commit '%s': %w", commitID.String(), err)
	}

	commitIDSplit := strings.Split(stdOut, " ")
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

// CreateCommitObject returns a commit object using the specified parameters.
func CreateCommitObject(gitConfig *config.Config, treeHash plumbing.Hash, parentHashes []plumbing.Hash, message string, clock clockwork.Clock) *object.Commit {
	author := object.Signature{
		Name:  gitConfig.User.Name,
		Email: gitConfig.User.Email,
		When:  clock.Now(),
	}

	commit := &object.Commit{
		Author:    author,
		Committer: author,
		TreeHash:  treeHash,
		Message:   message,
	}

	if len(parentHashes) > 0 {
		commit.ParentHashes = make([]plumbing.Hash, 0, len(parentHashes))
	}
	for _, parentHash := range parentHashes {
		if !parentHash.IsZero() {
			commit.ParentHashes = append(commit.ParentHashes, parentHash)
		}
	}

	return commit
}

// KnowsCommit indicates if the commit under test, identified by commitID, has a
// path to commit. If commit is the same as the commit under test or if commit
// is an ancestor of commit under test, KnowsCommit returns true.
func KnowsCommit(repo *git.Repository, commitID plumbing.Hash, commit *object.Commit) (bool, error) {
	if commitID == commit.Hash {
		return true, nil
	}

	commitUnderTest, err := GetCommit(repo, commitID)
	if err != nil {
		return false, err
	}

	return commit.IsAncestor(commitUnderTest)
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

	_, _, err := r.executeGitCommand("merge-base", "--is-ancestor", ancestorCommitID.String(), testCommitID.String())
	return err == nil, nil
}

// GetCommit returns the requested commit object.
func GetCommit(repo *git.Repository, commitID plumbing.Hash) (*object.Commit, error) {
	return repo.CommitObject(commitID)
}

func signCommit(commit *object.Commit) (string, error) {
	commitContents, err := getCommitBytesWithoutSignature(commit)
	if err != nil {
		return "", err
	}

	return signGitObject(commitContents)
}

// ensureIsCommit is a helper to check that the ID represents a Git commit
// object.
func (r *Repository) ensureIsCommit(commitID Hash) error {
	objType, err := r.executeGitCommandString("cat-file", "-t", commitID.String())
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
