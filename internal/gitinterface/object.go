// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"fmt"
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
	_, err := r.executor("cat-file", "-e", objectID.String()).executeString()
	return err == nil
}

func (r *Repository) GetObjectType(objectID Hash) (ObjectType, error) {
	objType, err := r.executor("cat-file", "-t", objectID.String()).executeString()
	if err != nil {
		return 0, fmt.Errorf("unable to inspect object type: %w", err)
	}

	switch objType {
	case "blob":
		return BlobObjectType, nil
	case "tree":
		return TreeObjectType, nil
	case "commit":
		return CommitObjectType, nil
	case "tag":
		return TagObjectType, nil
	default:
		return 0, ErrInvalidObjectType
	}
}
