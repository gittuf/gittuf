// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"sort"

	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing/object"
	"github.com/gittuf/gittuf/internal/third_party/go-git/storage/memory"
)

// WriteTree creates a Git tree with the specified entries. It sorts the entries
// prior to creating the tree.
func WriteTree(repo *git.Repository, entries []object.TreeEntry) (plumbing.Hash, error) {
	sort.Slice(entries, func(i int, j int) bool {
		return entries[i].Name < entries[j].Name
	})
	obj := repo.Storer.NewEncodedObject()
	tree := object.Tree{
		Entries: entries,
	}
	if err := tree.Encode(obj); err != nil {
		return plumbing.ZeroHash, err
	}
	return repo.Storer.SetEncodedObject(obj)
}

// GetTree returns the requested tree object.
func GetTree(repo *git.Repository, commitID plumbing.Hash) (*object.Tree, error) {
	return repo.TreeObject(commitID)
}

// EmptyTree returns the hash of an empty tree in a Git repository.
// Note: it is generated on the fly rather than stored as a constant to support
// SHA-256 repositories in future.
func EmptyTree() plumbing.Hash {
	obj := memory.NewStorage().NewEncodedObject()
	tree := object.Tree{}
	tree.Encode(obj) //nolint:errcheck

	return obj.Hash()
}
