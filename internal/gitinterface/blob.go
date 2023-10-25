// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"io"

	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	"github.com/gittuf/gittuf/internal/third_party/go-git/storage/memory"
)

var ErrWrittenBlobLengthMismatch = errors.New("length of blob written does not match length of contents")

// ReadBlob returns the contents of a the blob referenced by blobID.
func ReadBlob(repo *git.Repository, blobID plumbing.Hash) ([]byte, error) {
	blob, err := repo.BlobObject(blobID)
	if err != nil {
		return nil, err
	}

	reader, err := blob.Reader()
	if err != nil {
		return nil, err
	}

	return io.ReadAll(reader)
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

// EmptyBlob returns the hash of an empty blob in a Git repository.
// Note: it is generated on the fly rather than stored as a constant to support
// SHA-256 repositories in future.
func EmptyBlob() plumbing.Hash {
	obj := memory.NewStorage().NewEncodedObject()
	obj.SetType(plumbing.BlobObject)

	return obj.Hash()
}
