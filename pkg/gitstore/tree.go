// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitstore

import (
	"errors"

	"github.com/gittuf/gittuf/pkg/githash"
)

// ErrDuplicateTreePath is returned by WriteTree when two entries share a path.
var ErrDuplicateTreePath = errors.New("duplicate path in tree entries")

// EntryKind is whether a tree entry is a regular file blob or a grafted
// subtree.
type EntryKind uint8

const (
	KindBlob EntryKind = iota
	KindSubtree
)

// TreeEntry is a self-contained description of one entry to place in a tree:
// its path, the ID of the existing object it points to, and whether that
// object is a blob or a subtree. Intermediate trees implied by "/" in Path are
// created automatically. A zero-value Kind is KindBlob.
type TreeEntry struct {
	Path string
	ID   githash.Hash
	Kind EntryKind
}
