// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"fmt"
	"io"
	"strings"
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

// GetBlobID returns the ID of the blob at the specified path in the given
// reference. If the reference is ":", it will look for the blob at the path
// in the current working directory of the repository.
func (r *Repository) GetBlobID(ref, path string) (Hash, error) {
	var fullRef string
	if ref == ":" {
		fullRef = ":" + path
	} else {
		fullRef = ref + ":" + path
	}

	stdout, err := r.executor("rev-parse", fullRef).executeString()
	if err != nil {
		return ZeroHash, fmt.Errorf("unable to resolve blobID for %s in %s: %w", path, ref, err)
	}
	blobID, err := NewHash(strings.TrimSpace(stdout))
	if err != nil {
		return ZeroHash, fmt.Errorf("invalid blob id: %w", err)
	}
	return blobID, nil
}
