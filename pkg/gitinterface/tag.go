// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/go-git/go-git/v6/plumbing"
	"github.com/go-git/go-git/v6/plumbing/object"
	"github.com/go-git/go-git/v6/storage/memory"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
)

var ErrTagAlreadyExists = errors.New("tag already exists")

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
	// Git appends tag signatures to the tag payload regardless of the object
	// format; only commits store the signature under a header named for the
	// hash algorithm (`gpgsig` / `gpgsig-sha256`).
	tag.Signature = signature

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
	targetID, err := r.executor("rev-list", "-n", "1", tagID.String()).executeString()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to resolve tag's target ID: %w", err)
	}

	hash, err := NewHash(targetID)
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid format for target ID: %w", err)
	}

	return hash, nil
}

// verifyTagSignature verifies a signature for the specified tag using the
// provided public key.
func (r *Repository) verifyTagSignature(ctx context.Context, tagID Hash, key *signerverifier.SSLibKey) error {
	goGitRepo, err := r.GetGoGitRepository()
	if err != nil {
		return fmt.Errorf("error opening repository: %w", err)
	}

	tag, err := goGitRepo.TagObject(plumbing.NewHash(tagID.String()))
	if err != nil {
		return fmt.Errorf("unable to load tag object: %w", err)
	}

	tagContents, err := getTagBytesWithoutSignature(tag)
	if err != nil {
		return fmt.Errorf("unable to encode tag contents for verification: %w", err)
	}

	// Git appends tag signatures to the tag payload regardless of the object
	// format, so the signature is always in the Signature field (unlike
	// commits, where the header depends on the hash algorithm).
	tagSignature := tag.Signature

	if signatureBlockCount(tagSignature) > 1 {
		return errors.Join(ErrIncorrectVerificationKey, ErrMultipleSignatures)
	}

	switch key.KeyType {
	case gpg.KeyType:
		verifier, err := gpg.NewVerifierFromKey(key)
		if err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}
		if err := verifier.Verify(ctx, tagContents, []byte(tagSignature)); err != nil {
			return ErrIncorrectVerificationKey
		}

		return nil
	case ssh.KeyType:
		if err := verifySSHKeySignature(ctx, key, tagContents, []byte(tagSignature)); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	case sigstore.KeyType:
		if err := verifyGitsignSignature(ctx, r, key, tagContents, []byte(tagSignature)); err != nil {
			return errors.Join(ErrIncorrectVerificationKey, err)
		}

		return nil
	}

	return ErrUnknownSigningMethod
}

func (r *Repository) ensureIsTag(tagID Hash) error {
	objType, err := r.executor("cat-file", "-t", tagID.String()).executeString()
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
