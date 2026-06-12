// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"io"

	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
)

// ReadBlob returns the contents of the blob referenced by blobID.
func (r *Repository) ReadBlob(blobID Hash) ([]byte, error) {
	repo, err := r.GetGoGitRepository()
	if err != nil {
		return nil, err
	}

	h := plumbing.NewHash(blobID.String())
	obj, err := repo.Object(plumbing.AnyObject, h)
	if err != nil {
		return nil, fmt.Errorf("unable to inspect if object is blob: %w", err)
	}

	blobObj, ok := obj.(*object.Blob)
	if !ok {
		return nil, fmt.Errorf("requested Git ID '%s' is not a blob object", blobID.String())
	}

	reader, err := blobObj.Reader()
	if err != nil {
		return nil, fmt.Errorf("unable to read blob: %w", err)
	}
	defer reader.Close()

	return io.ReadAll(reader)
}

// WriteBlob creates a blob object with the specified contents and returns the
// ID of the resultant blob.
func (r *Repository) WriteBlob(contents []byte) (Hash, error) {
	repo, err := r.GetGoGitRepository()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to write blob: %w", err)
	}

	obj := repo.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)

	writer, err := obj.Writer()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to write blob: %w", err)
	}

	if _, err := writer.Write(contents); err != nil {
		writer.Close()
		return ZeroHash, fmt.Errorf("unable to write blob: %w", err)
	}
	writer.Close()

	hash, err := repo.Storer.SetEncodedObject(obj)
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to write blob: %w", err)
	}

	gitHash, err := NewHash(hash.String())
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid Git ID for blob: %w", err)
	}

	return gitHash, nil
}
