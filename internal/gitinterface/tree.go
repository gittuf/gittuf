package gitinterface

import (
	"sort"

	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/object"
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
