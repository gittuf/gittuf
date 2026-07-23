// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Package gitstore defines the Git storage interface gittuf's packages
// consume. It depends only on the standard library and pkg/githash, so
// dependency-light packages (rsl, and eventually policy) can name their
// storage without importing gitinterface's signing/attestation stack.
// *gitinterface.Repository satisfies Storer structurally. Other backends
// (e.g. go-git) can implement it directly.
package gitstore

import (
	"errors"

	"github.com/gittuf/gittuf/pkg/githash"
)

// ErrReferenceNotFound must be returned (via errors.Is) by a Storer's
// GetReference when the requested reference does not exist.
// gitinterface.ErrReferenceNotFound is an alias of this sentinel.
var ErrReferenceNotFound = errors.New("requested Git reference not found")

// Storer is the union of Git storage operations gittuf's consumers need.
type Storer interface {
	// GetReference returns the tip of the specified reference, or an error
	// matching ErrReferenceNotFound if the reference does not exist.
	GetReference(refName string) (githash.Hash, error)

	// SetReference sets the specified reference to the provided Git ID.
	SetReference(refName string, gitID githash.Hash) error

	// DeleteReference deletes the specified reference.
	DeleteReference(refName string) error

	// ReadBlob returns the contents of the specified blob.
	ReadBlob(blobID githash.Hash) ([]byte, error)

	// WriteBlob writes the contents as a blob and returns its ID.
	WriteBlob(contents []byte) (githash.Hash, error)

	// EmptyTree returns the ID of the empty tree.
	EmptyTree() (githash.Hash, error)

	// WriteTree writes a tree from blobs and subtrees keyed by their
	// slash-separated path, creating intermediate trees as needed. Blob
	// entries get default permissions. Subtree values are IDs of existing
	// trees grafted at their path. The returned tree ID must be
	// deterministic and independent of the map iteration order, so an
	// implementation must impose a canonical entry order (Git's own tree
	// object format requires sorted entries).
	WriteTree(blobs, subtrees map[string]githash.Hash) (githash.Hash, error)

	// GetAllFilesInTree returns the recursively flattened path → blobID
	// mapping of the specified tree.
	GetAllFilesInTree(treeID githash.Hash) (map[string]githash.Hash, error)

	// GetTreeItems returns the immediate children of the specified tree,
	// names mapped to IDs, including subtrees.
	GetTreeItems(treeID githash.Hash) (map[string]githash.Hash, error)

	// GetPathIDInTree returns the ID of the object at the specified
	// slash-separated path within the specified tree.
	GetPathIDInTree(treePath string, treeID githash.Hash) (githash.Hash, error)

	// GetCommitTreeID returns the ID of the specified commit's tree.
	GetCommitTreeID(commitID githash.Hash) (githash.Hash, error)

	// GetCommitMessage returns the specified commit's message.
	GetCommitMessage(commitID githash.Hash) (string, error)

	// GetCommitParentIDs returns the IDs of the specified commit's parents.
	GetCommitParentIDs(commitID githash.Hash) ([]githash.Hash, error)

	// GetCommitsBetweenRange returns the commits reachable from commitNewID
	// but not from commitOldID. A zero commitOldID (nil, empty, or the
	// all-zeroes hash of either object format) means no lower bound, and
	// all commits reachable from commitNewID are returned.
	GetCommitsBetweenRange(commitNewID, commitOldID githash.Hash) ([]githash.Hash, error)

	// GetFilePathsChangedByCommit returns the paths changed by the specified
	// commit relative to its parents.
	GetFilePathsChangedByCommit(commitID githash.Hash) ([]string, error)

	// KnowsCommit reports whether ancestorID is an ancestor of commitID.
	KnowsCommit(commitID, ancestorID githash.Hash) (bool, error)

	// GetMergeTree returns the tree resulting from merging the two commits.
	// This is the hardest method for backends not built on the git binary.
	GetMergeTree(commitAID, commitBID githash.Hash) (githash.Hash, error)

	// GetTagTarget returns the ID of the object the specified tag points to.
	GetTagTarget(tagID githash.Hash) (githash.Hash, error)

	// GetObjectSignature returns the signed payload and detached signature
	// of the specified commit or tag. The signature is empty when the object
	// is unsigned.
	GetObjectSignature(objectID githash.Hash) ([]byte, []byte, error)

	// Commit commits the specified tree to targetRef, signing it per the
	// store's configuration when sign is true, and returns the commit ID.
	Commit(treeID githash.Hash, targetRef, message string, sign bool) (githash.Hash, error)

	// CommitUsingSpecificKey is Commit signing with the provided PEM encoded
	// key. Intended for gittuf's developer mode and tests.
	CommitUsingSpecificKey(treeID githash.Hash, targetRef, message string, signingKeyPEMBytes []byte) (githash.Hash, error)

	// ZeroHash returns the all-zeroes hash matching the store's object
	// format.
	ZeroHash() githash.Hash

	// GetGitConfig returns the store's Git configuration.
	GetGitConfig() (map[string]string, error)

	// ResetDueToError force-resets the specified reference to commitID and
	// returns cause, wrapped if the reset itself fails.
	ResetDueToError(cause error, refName string, commitID githash.Hash) error
}
