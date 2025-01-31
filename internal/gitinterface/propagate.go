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

func (r *Repository) PropagateUpstreamRepositoryContents(upstream *Repository, upstreamCommitID Hash, refDirectoryMapping map[string]string) (map[string]Hash, error) {
	treeID, err := upstream.GetCommitTreeID(upstreamCommitID)
	if err != nil {
		return nil, err
	}
	filesToCopy, err := upstream.GetAllFilesInTree(treeID)
	if err != nil {
		return nil, err
	}

	for _, blobID := range filesToCopy {
		blob, err := upstream.ReadBlob(blobID)
		if err != nil {
			return nil, err
		}
		localBlobID, err := r.WriteBlob(blob)
		if err != nil {
			return nil, err
		}
		if !localBlobID.Equal(blobID) {
			return nil, ErrCopyingBlobIDsDoNotMatch
		}
	}

	treeBuilder := NewTreeBuilder(r)
	refHashes := map[string]Hash{}
	for ref, directory := range refDirectoryMapping {
		currentTip, err := r.GetReference(ref)
		if err != nil {
			return nil, err
		}
		currentRefTree, err := r.GetCommitTreeID(currentTip)
		if err != nil {
			return nil, err
		}
		currentFiles, err := r.GetAllFilesInTree(currentRefTree)
		if err != nil {
			return nil, err
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
			return nil, err
		}

		hash, err := r.Commit(newTreeID, ref, fmt.Sprintf("Updating contents of %s", strings.TrimPrefix(directory, "/")), false)
		if err != nil {
			return nil, err
		}

		refHashes[ref] = hash
	}

	return refHashes, nil
}
