// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"errors"
	"fmt"
	"io"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
)

var ErrWrittenBlobLengthMismatch = errors.New("length of blob written does not match length of contents")

// ReadBlob returns the contents of a the blob referenced by blobID.
func ReadBlob(repo *git.Repository, blobID plumbing.Hash) ([]byte, error) {
	blob, err := GetBlob(repo, blobID)
	if err != nil {
		return nil, err
	}

	reader, err := blob.Reader()
	if err != nil {
		return nil, err
	}

	return io.ReadAll(reader)
}

// ReadBlob returns the contents of a the blob referenced by blobID.
func (r *Repository) ReadBlob(blobID Hash) ([]byte, error) {
	objType, err := r.executeGitCommandString("cat-file", "-t", blobID.String())
	if err != nil {
		return nil, fmt.Errorf("unable to inspect if object is blob: %w", err)
	} else if objType != "blob" {
		return nil, fmt.Errorf("requested Git ID '%s' is not a blob object", blobID.String())
	}

	stdOut, stdErr, err := r.executeGitCommand("cat-file", "-p", blobID.String())
	if err != nil {
		return nil, fmt.Errorf("unable to read blob: %s", stdErr)
	}

	return io.ReadAll(stdOut)
}

// WriteBlob creates a blob object with the specified contents and returns the
// ID of the resultant blob.
func WriteBlob(repo *git.Repository, contents []byte) (plumbing.Hash, error) {
	obj := repo.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)

	writer, err := obj.Writer()
	if err != nil {
		return plumbing.ZeroHash, err
	}

	length, err := writer.Write(contents)
	if err != nil {
		return plumbing.ZeroHash, err
	}

	if length != len(contents) {
		return plumbing.ZeroHash, ErrWrittenBlobLengthMismatch
	}

	return repo.Storer.SetEncodedObject(obj)
}

// WriteBlob creates a blob object with the specified contents and returns the
// ID of the resultant blob.
func (r *Repository) WriteBlob(contents []byte) (Hash, error) {
	stdInBuf := bytes.NewBuffer(contents)
	objID, err := r.executeGitCommandWithStdInString(stdInBuf, "hash-object", "-t", "blob", "-w", "--stdin")
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to write blob: %w", err)
	}

	hash, err := NewHash(objID)
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid Git ID for blob: %w", err)
	}

	return hash, nil
}

// GetBlob returns the requested blob object.
func GetBlob(repo *git.Repository, blobID plumbing.Hash) (*object.Blob, error) {
	return repo.BlobObject(blobID)
}

// EmptyBlob returns the hash of an empty blob in a Git repository.
// Note: it is generated on the fly rather than stored as a constant to support
// SHA-256 repositories in future.
func EmptyBlob() plumbing.Hash {
	obj := memory.NewStorage().NewEncodedObject()
	obj.SetType(plumbing.BlobObject)

	return obj.Hash()
}
