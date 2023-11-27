// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"errors"
	"io"

	"github.com/gittuf/gittuf/internal/signerverifier"
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
	case signerverifier.FulcioKeyType:
		commitContents, err := getCommitBytesWithoutSignature(commit)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		commitSignature := []byte(commit.PGPSignature)

		return verifyGitsignSignature(ctx, key, commitContents, commitSignature)
	}

	return ErrUnknownSigningMethod
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
