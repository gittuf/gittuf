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
	ErrCopyingBlobIDsDoNotMatch    = errors.New("blob ID in local repository does not match upstream repository")
	ErrCannotPropagateIntoRootTree = errors.New("propagation target cannot be empty or root of tree")
)

func (r *Repository) PropagateUpstreamRepositoryContents(upstream *Repository, upstreamCommitID Hash, localRef, localPath string) (Hash, error) {
	if localPath == "" {
		return nil, ErrCannotPropagateIntoRootTree
	}
	currentTip, err := r.GetReference(localRef)
	if err != nil {
		if !errors.Is(err, ErrReferenceNotFound) {
			return nil, err
		}
	}

	entries := []TreeEntry{}
	if !currentTip.IsZero() {
		currentRefTree, err := r.GetCommitTreeID(currentTip)
		if err != nil {
			return nil, err
		}
		currentFiles, err := r.GetAllFilesInTree(currentRefTree)
		if err != nil {
			return nil, err
		}

		// Ignore entries for `localPath` to account for upstream deletions
		// If localPath is foo/, we want to ignore all items under foo/
		// If localPath is foo, we want to ignore all items under foo/
		// If localPath is foo, we DO NOT want to remove all items under foobar/
		// So, add the / suffix if necessary to localPath
		if !strings.HasSuffix(localPath, "/") {
			localPath += "/"
		}

		// Create list of TreeEntry objects representing all blobs except those
		// currently under localPath
		for filePath, blobID := range currentFiles {
			if !strings.HasPrefix(filePath, localPath) {
				entries = append(entries, NewEntryBlob(filePath, blobID))
			}
		}
	}

	// Remove trailing "/" now
	localPath = strings.TrimSuffix(localPath, "/")

	treeID, err := upstream.GetCommitTreeID(upstreamCommitID)
	if err != nil {
		return nil, err
	}

	if r.HasObject(treeID) {
		// Use existing intermediate tree
		entries = append(entries, NewEntryTree(localPath, treeID))
	} else {
		// We have to create the intermediate tree for localPath
		filesToCopy, err := upstream.GetAllFilesInTree(treeID)
		if err != nil {
			return nil, err
		}

		for blobPath, blobID := range filesToCopy {
			// if blob already exists, we don't need to carry out expensive
			// read/write
			if !r.HasObject(blobID) {
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

			// add blob to entries, with the path including the localPath prefix
			entries = append(entries, NewEntryBlob(path.Join(localPath, blobPath), blobID))
		}
	}

	treeBuilder := NewTreeBuilder(r)
	newTreeID, err := treeBuilder.WriteTreeFromEntries(entries)
	if err != nil {
		return nil, err
	}

	return r.Commit(newTreeID, localRef, fmt.Sprintf("Update contents of '%s'\n", localPath), false)
}
