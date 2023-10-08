// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"errors"
	"io"
	"strings"

	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jonboulle/clockwork"
)

var (
	ErrTagAlreadyExists = errors.New("tag already exists")
)

// IsTag returns true if the specified target is a tag in the repository.
func IsTag(repo *git.Repository, target string) bool {
	absPath, err := AbsoluteReference(repo, target)
	if err == nil {
		if strings.HasPrefix(absPath, TagRefPrefix) {
			return true
		}
	}

	_, err = repo.TagObject(plumbing.NewHash(target))
	return err == nil
}

// Tag creates a new tag in the repository pointing to the specified target.
func Tag(repo *git.Repository, target plumbing.Hash, name, message string, sign bool) (plumbing.Hash, error) {
	gitConfig, err := getGitConfig(repo)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	_, err = repo.Reference(plumbing.NewTagReferenceName(name), true)
	if err == nil {
		return plumbing.ZeroHash, ErrTagAlreadyExists
	}

	targetObj, err := repo.Object(plumbing.AnyObject, target)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	tag := CreateTagObject(gitConfig, targetObj, name, message, clock)

	if sign {
		signature, err := signTag(tag)
		if err != nil {
			return plumbing.ZeroHash, err
		}
		tag.PGPSignature = signature
	}

	return ApplyTag(repo, tag)
}

// ApplyTag sets the tag reference after the tag object is written to the
// repository's object store.
func ApplyTag(repo *git.Repository, tag *object.Tag) (plumbing.Hash, error) {
	tagHash, err := WriteTag(repo, tag)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	newRef := plumbing.NewHashReference(plumbing.NewTagReferenceName(tag.Name), tagHash)
	return tagHash, repo.Storer.SetReference(newRef)
}

// WriteTag writes the tag to the repository's object store.
func WriteTag(repo *git.Repository, tag *object.Tag) (plumbing.Hash, error) {
	obj := repo.Storer.NewEncodedObject()
	if err := tag.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}

	return repo.Storer.SetEncodedObject(obj)
}

// CreateTagObject crafts and returns a new tag object using the specified
// parameters.
func CreateTagObject(gitConfig *config.Config, targetObj object.Object, name, message string, clock clockwork.Clock) *object.Tag {
	return &object.Tag{
		Name: name,
		Tagger: object.Signature{
			Name:  gitConfig.User.Name,
			Email: gitConfig.User.Email,
			When:  clock.Now(),
		},
		Message:    message,
		TargetType: targetObj.Type(),
		Target:     targetObj.ID(),
	}
}

// VerifyTagSignature is used to verify a cryptographic signature associated
// with tag using TUF public keys.
func VerifyTagSignature(ctx context.Context, tag *object.Tag, key *tuf.Key) error {
	switch key.KeyType {
	case signerverifier.GPGKeyType:
		if _, err := tag.Verify(key.KeyVal.Public); err != nil {
			return ErrIncorrectVerificationKey
		}

		return nil
	case signerverifier.FulcioKeyType:
		tagContents, err := getTagBytesWithoutSignature(tag)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		tagSignature := []byte(tag.PGPSignature)

		return verifyGitsignSignature(ctx, key, tagContents, tagSignature)
	}

	return ErrUnknownSigningMethod
}

func signTag(tag *object.Tag) (string, error) {
	tagContents, err := getTagBytesWithoutSignature(tag)
	if err != nil {
		return "", err
	}

	return signGitObject(tagContents)
}

func getTagBytesWithoutSignature(tag *object.Tag) ([]byte, error) {
	tagEncoded := memory.NewStorage().NewEncodedObject()
	if err := tag.EncodeWithoutSignature(tagEncoded); err != nil {
		return nil, err
	}
	r, err := tagEncoded.Reader()
	if err != nil {
		return nil, err
	}

	return io.ReadAll(r)
}
