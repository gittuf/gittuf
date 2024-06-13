// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

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

	_, err = GetTag(repo, plumbing.NewHash(target))
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

// TagUsingSpecificKey creates a Git tag signed using the specified, PEM encoded
// SSH or GPG key. It is primarily intended for use with testing. As of now,
// gittuf is not expected to be used to create tags in developer workflows,
// though this may change with command compatibility.
func (r *Repository) TagUsingSpecificKey(target Hash, name, message string, signingKeyPEMBytes []byte) (Hash, error) {
	gitConfig, err := r.GetGitConfig()
	if err != nil {
		return ZeroHash, err
	}

	goGitRepo, err := r.GetGoGitRepository()
	if err != nil {
		return ZeroHash, err
	}

	targetObj, err := goGitRepo.Object(plumbing.AnyObject, plumbing.NewHash(target.String()))
	if err != nil {
		return ZeroHash, err
	}

	if !strings.HasSuffix(message, "\n") {
		message += "\n"
	}

	tag := &object.Tag{
		Name: name,
		Tagger: object.Signature{
			Name:  gitConfig["user.name"],
			Email: gitConfig["user.email"],
			When:  r.clock.Now(),
		},
		Message:    message,
		TargetType: targetObj.Type(),
		Target:     targetObj.ID(),
	}

	tagContents, err := getTagBytesWithoutSignature(tag)
	if err != nil {
		return ZeroHash, err
	}
	signature, err := signGitObjectUsingKey(tagContents, signingKeyPEMBytes)
	if err != nil {
		return ZeroHash, err
	}
	tag.PGPSignature = signature

	obj := goGitRepo.Storer.NewEncodedObject()
	if err := tag.Encode(obj); err != nil {
		return ZeroHash, err
	}
	tagID, err := goGitRepo.Storer.SetEncodedObject(obj)
	if err != nil {
		return ZeroHash, err
	}

	tagIDHash, err := NewHash(tagID.String())
	if err != nil {
		return ZeroHash, err
	}

	return tagIDHash, r.SetReference(TagReferenceName(name), tagIDHash)
}

// GetTagTarget returns the ID of the Git object a tag points to.
func (r *Repository) GetTagTarget(tagID Hash) (Hash, error) {
	targetID, err := r.executeGitCommandString("rev-list", "-n", "1", tagID.String())
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to resolve tag's target ID: %w", err)
	}

	hash, err := NewHash(targetID)
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid format for target ID: %w", err)
	}

	return hash, nil
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
	case signerverifier.RSAKeyType, signerverifier.ECDSAKeyType, signerverifier.ED25519KeyType, ssh.SSHKeyType:
		tagContents, err := getTagBytesWithoutSignature(tag)
		if err != nil {
			return errors.Join(ErrVerifyingSSHSignature, err)
		}
		tagSignature := []byte(tag.PGPSignature)

		if err := verifySSHKeySignature(key, tagContents, tagSignature); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	case signerverifier.FulcioKeyType:
		tagContents, err := getTagBytesWithoutSignature(tag)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		tagSignature := []byte(tag.PGPSignature)

		if err := verifyGitsignSignature(ctx, key, tagContents, tagSignature); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	}

	return ErrUnknownSigningMethod
}

// verifyTagSignature verifies a signature for the specified tag using the
// provided public key.
func (r *Repository) verifyTagSignature(ctx context.Context, tagID Hash, key *tuf.Key) error {
	goGitRepo, err := r.GetGoGitRepository()
	if err != nil {
		return fmt.Errorf("error opening repository: %w", err)
	}

	tag, err := goGitRepo.TagObject(plumbing.NewHash(tagID.String()))
	if err != nil {
		return fmt.Errorf("unable to load commit object: %w", err)
	}

	switch key.KeyType {
	case signerverifier.GPGKeyType:
		if _, err := tag.Verify(key.KeyVal.Public); err != nil {
			return ErrIncorrectVerificationKey
		}

		return nil
	case signerverifier.RSAKeyType, signerverifier.ECDSAKeyType, signerverifier.ED25519KeyType, ssh.SSHKeyType:
		tagContents, err := getTagBytesWithoutSignature(tag)
		if err != nil {
			return errors.Join(ErrVerifyingSSHSignature, err)
		}
		tagSignature := []byte(tag.PGPSignature)

		if err := verifySSHKeySignature(key, tagContents, tagSignature); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	case signerverifier.FulcioKeyType:
		tagContents, err := getTagBytesWithoutSignature(tag)
		if err != nil {
			return errors.Join(ErrVerifyingSigstoreSignature, err)
		}
		tagSignature := []byte(tag.PGPSignature)

		if err := verifyGitsignSignature(ctx, key, tagContents, tagSignature); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	}

	return ErrUnknownSigningMethod
}

// GetTag returns the requested tag object.
func GetTag(repo *git.Repository, tagID plumbing.Hash) (*object.Tag, error) {
	return repo.TagObject(tagID)
}

func signTag(tag *object.Tag) (string, error) {
	tagContents, err := getTagBytesWithoutSignature(tag)
	if err != nil {
		return "", err
	}

	return signGitObject(tagContents)
}

func (r *Repository) ensureIsTag(tagID Hash) error {
	objType, err := r.executeGitCommandString("cat-file", "-t", tagID.String())
	if err != nil {
		return fmt.Errorf("unable to inspect if object is tag: %w", err)
	} else if objType != "tag" {
		return fmt.Errorf("requested Git ID '%s' is not a tag object", tagID.String())
	}

	return nil
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
