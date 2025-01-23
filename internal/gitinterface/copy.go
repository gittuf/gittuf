// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"fmt"
	"path"
	"strings"
)

var (
	ErrCopyingBlobIDsDoNotMatch = errors.New("blob ID in local repository does not match upstream repository")
)

func (r *Repository) CopyTreeFromRepositoryToPathInRef(upstream *Repository, upstreamCommitID Hash, refDirectoryMapping map[string]string) error {
	treeID, err := upstream.GetCommitTreeID(upstreamCommitID)
	if err != nil {
		return err
	}
	filesToCopy, err := upstream.GetAllFilesInTree(treeID)
	if err != nil {
		return err
	}

	for _, blobID := range filesToCopy {
		blob, err := upstream.ReadBlob(blobID)
		if err != nil {
			return err
		}
		localBlobID, err := r.WriteBlob(blob)
		if err != nil {
			return err
		}
		if !localBlobID.Equal(blobID) {
			return ErrCopyingBlobIDsDoNotMatch
		}
	}

	treeBuilder := NewTreeBuilder(r)
	for ref, directory := range refDirectoryMapping {
		currentTip, err := r.GetReference(ref)
		if err != nil {
			return err
		}
		currentRefTree, err := r.GetCommitTreeID(currentTip)
		if err != nil {
			return err
		}
		currentFiles, err := r.GetAllFilesInTree(currentRefTree)
		if err != nil {
			return err
		}

		// Delete entries for `directory` to account for deletions
		// If directory is foo/, we want to remove all items under foo/
		// If directory is foo, we want to remove all items under foo/
		// If directory is foo, we DO NOT want to remove all items under foobar/
		if !strings.HasSuffix(directory, "/") {
			directory += "/"
		}
		for filePath := range currentFiles {
			if strings.HasPrefix(filePath, directory) {
				delete(currentFiles, filePath)
			}
		}

		for filePath, blobID := range filesToCopy {
			// Add the specified directory as the prefix in the local repo
			currentFiles[path.Join(directory, filePath)] = blobID
		}

		newTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(currentFiles)
		if err != nil {
			return err
		}

		_, err = r.Commit(newTreeID, ref, fmt.Sprintf("Updating contents of %s", directory), false)
		if err != nil {
			return err
		}
	}

	return nil
}
