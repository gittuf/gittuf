// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"fmt"
	"io"
)

// ReadBlob returns the contents of the blob referenced by blobID.
func (r *Repository) ReadBlob(blobID Hash) ([]byte, error) {
	objType, err := r.executor("cat-file", "-t", blobID.String()).executeString()
	if err != nil {
		return nil, fmt.Errorf("unable to inspect if object is blob: %w", err)
	} else if objType != "blob" {
		return nil, fmt.Errorf("requested Git ID '%s' is not a blob object", blobID.String())
	}

	stdOut, stdErr, err := r.executor("cat-file", "-p", blobID.String()).execute()
	if err != nil {
		return nil, fmt.Errorf("unable to read blob: %s", stdErr)
	}

	return io.ReadAll(stdOut)
}

// ReadBlobFromString is a temporary function for when the argument being passed in is a string.
// This is seen in the hooks loading use-case.
// ReadBlob should probably be changed to accommodate both types
func (r *Repository) ReadBlobFromString(blobID string) ([]byte, error) {
	objType, err := r.executor("cat-file", "-t", blobID).executeString()
	if err != nil {
		return nil, fmt.Errorf("unable to inspect if object is blob: %w", err)
	} else if objType != "blob" {
		return nil, fmt.Errorf("requested Git ID '%s' is not a blob object", blobID)
	}

	stdOut, stdErr, err := r.executor("cat-file", "-p", blobID).execute()
	if err != nil {
		return nil, fmt.Errorf("unable to read blob: %s", stdErr)
	}

	return io.ReadAll(stdOut)
}

// WriteBlob creates a blob object with the specified contents and returns the
// ID of the resultant blob.
func (r *Repository) WriteBlob(contents []byte) (Hash, error) {
	stdInBuf := bytes.NewBuffer(contents)
	objID, err := r.executor("hash-object", "-t", "blob", "-w", "--stdin").withStdIn(stdInBuf).executeString()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to write blob: %w", err)
	}

	hash, err := NewHash(objID)
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid Git ID for blob: %w", err)
	}

	return hash, nil
}
