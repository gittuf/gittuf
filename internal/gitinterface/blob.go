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

// IsBlobBinary returns whether Git thinks the specified blob is a binary file,
// as opposed to a text file.
func (r *Repository) IsBlobBinary(blobID Hash) (bool, error) {
	objType, err := r.executor("cat-file", "-t", blobID.String()).executeString()
	if err != nil {
		return false, fmt.Errorf("unable to inspect if object is blob: %w", err)
	} else if objType != "blob" {
		return false, fmt.Errorf("requested Git ID '%s' is not a blob object", blobID.String())
	}

	emptyBlobHash, err := r.WriteBlob(nil)
	if err != nil {
		return false, err
	}

	// Git 2.43.0 added support for specifying blob IDs to merge-file. On older
	// Git versions, we have to make do with parsing the output of running git
	// diff with --numstat.
	isNiceGit, err := isNiceGitVersion()
	if err != nil {
		return false, err
	}

	if !isNiceGit {
		output, err := r.executor("diff", "--numstat", emptyBlobHash.String(), blobID.String()).executeString()
		if err != nil {
			return false, err
		}

		//
		if strings.Compare(string(output[0]), "-") == 0 {
			return true, nil
		}

		return false, nil
	}

	// Method taken from https://stackoverflow.com/questions/6119956/how-to-determine-if-git-handles-a-file-as-binary-or-as-text
	_, err = r.executor("merge-file", "--object-id", "-p", emptyBlobHash.String(), emptyBlobHash.String(), blobID.String()).executeString()
	return err != nil, nil
}
