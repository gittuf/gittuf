// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"errors"
	"io"
	"path"
	"sort"
	"strings"

	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing/filemode"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing/object"
	"github.com/gittuf/gittuf/internal/third_party/go-git/storage/memory"
)

var ErrNoEntries = errors.New("no entries specified to write tree")

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
func GetTree(repo *git.Repository, treeID plumbing.Hash) (*object.Tree, error) {
	return repo.TreeObject(treeID)
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

// GetAllFilesInTree returns all filepaths and the corresponding hash in the
// specified tree.
func GetAllFilesInTree(tree *object.Tree) (map[string]plumbing.Hash, error) {
	treeWalker := object.NewTreeWalker(tree, true, nil)
	defer treeWalker.Close()

	files := map[string]plumbing.Hash{}

	for {
		// This is based on FilesIter in go-git. That implementation loads each
		// blob which we don't want to do.
		name, entry, err := treeWalker.Next()
		if err != nil {
			if errors.Is(err, io.EOF) {
				break
			}
			return nil, err
		}

		if entry.Mode == filemode.Dir || entry.Mode == filemode.Submodule {
			continue
		}

		files[name] = entry.Hash
	}

	return files, nil
}

// TreeBuilder is used to create multi-level trees in a repository.
// Based on `buildTreeHelper` in go-git.
type TreeBuilder struct {
	r       *git.Repository
	trees   map[string]*object.Tree
	entries map[string]*object.TreeEntry
}

// NewTreeBuilder returns a TreeBuilder instance for the repository.
func NewTreeBuilder(repo *git.Repository) *TreeBuilder {
	return &TreeBuilder{r: repo}
}

// WriteRootTreeFromBlobIDs accepts a map of paths to their blob IDs and returns
// the root tree ID that contains these files.
func (t *TreeBuilder) WriteRootTreeFromBlobIDs(files map[string]plumbing.Hash) (plumbing.Hash, error) {
	if len(files) == 0 {
		return plumbing.ZeroHash, ErrNoEntries
	}

	rootNodeKey := ""
	t.trees = map[string]*object.Tree{rootNodeKey: {}}
	t.entries = map[string]*object.TreeEntry{}

	for path, blobID := range files {
		t.buildIntermediates(path, blobID)
	}

	return t.writeTrees(rootNodeKey, t.trees[rootNodeKey])
}

// buildIntermediates identifies the intermediate trees that must be constructed
// for the specified path.
func (t *TreeBuilder) buildIntermediates(name string, blobID plumbing.Hash) {
	parts := strings.Split(name, "/")

	var fullPath string
	for _, part := range parts {
		parent := fullPath
		fullPath = path.Join(fullPath, part)

		t.buildTree(name, parent, fullPath, blobID)
	}
}

// buildTree populates tree and entry information for each tree that must be
// created.
func (t *TreeBuilder) buildTree(name, parent, fullPath string, blobID plumbing.Hash) {
	if _, ok := t.trees[fullPath]; ok {
		return
	}

	if _, ok := t.entries[fullPath]; ok {
		return
	}

	entry := object.TreeEntry{Name: path.Base(fullPath)}

	if fullPath == name {
		entry.Mode = filemode.Regular
		entry.Hash = blobID
	} else {
		entry.Mode = filemode.Dir
		t.trees[fullPath] = &object.Tree{}
	}

	t.trees[parent].Entries = append(t.trees[parent].Entries, entry)
}

// writeTrees recursively stores each tree that must be created in the
// repository's object store. It returns the ID of the tree created at each
// invocation.
func (t *TreeBuilder) writeTrees(parent string, tree *object.Tree) (plumbing.Hash, error) {
	for i, e := range tree.Entries {
		if e.Mode != filemode.Dir && !e.Hash.IsZero() {
			continue
		}

		p := path.Join(parent, e.Name)
		entryID, err := t.writeTrees(p, t.trees[p])
		if err != nil {
			return plumbing.ZeroHash, err
		}
		e.Hash = entryID

		tree.Entries[i] = e
	}

	return WriteTree(t.r, tree.Entries)
}
