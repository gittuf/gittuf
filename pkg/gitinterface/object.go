// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"fmt"

	"github.com/go-git/go-git/v5/plumbing"
)

type ObjectType uint

const (
	BlobObjectType ObjectType = iota + 1
	TreeObjectType
	CommitObjectType
	TagObjectType
)

var ErrInvalidObjectType = errors.New("unknown Git object type")

// HasObject returns true if an object with the specified Git ID exists in the
// repository.
func (r *Repository) HasObject(objectID Hash) bool {
	repo, err := r.GetGoGitRepository()
	if err != nil {
		return false
	}
	_, err = repo.Storer.EncodedObject(plumbing.AnyObject, plumbing.NewHash(objectID.String()))
	return err == nil
}

func (r *Repository) GetObjectType(objectID Hash) (ObjectType, error) {
	repo, err := r.GetGoGitRepository()
	if err != nil {
		return 0, fmt.Errorf("unable to inspect object type: %w", err)
	}
	obj, err := repo.Storer.EncodedObject(plumbing.AnyObject, plumbing.NewHash(objectID.String()))
	if err != nil {
		return 0, fmt.Errorf("unable to inspect object type: %w", err)
	}

	switch obj.Type() {
	case plumbing.BlobObject:
		return BlobObjectType, nil
	case plumbing.TreeObject:
		return TreeObjectType, nil
	case plumbing.CommitObject:
		return CommitObjectType, nil
	case plumbing.TagObject:
		return TagObjectType, nil
	default:
		return 0, ErrInvalidObjectType
	}
}

// GetObjectSize returns the size of the object with the specified Git ID.
func (r *Repository) GetObjectSize(objectID Hash) (uint64, error) {
	repo, err := r.GetGoGitRepository()
	if err != nil {
		return 0, fmt.Errorf("unable to inspect object size: %w", err)
	}
	obj, err := repo.Storer.EncodedObject(plumbing.AnyObject, plumbing.NewHash(objectID.String()))
	if err != nil {
		return 0, fmt.Errorf("unable to inspect object size: %w", err)
	}
	return uint64(obj.Size()), nil
}
