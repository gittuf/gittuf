// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing/filemode"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing/object"
	"github.com/gittuf/gittuf/internal/third_party/go-git/storage/memory"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
)

func TestWriteTree(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	readContents := []byte("test file read")
	readHash, err := WriteBlob(repo, readContents)
	if err != nil {
		t.Fatal(err)
	}

	writeContents := []byte("test file write")
	writeHash, err := WriteBlob(repo, writeContents)
	if err != nil {
		t.Fatal(err)
	}

	entries := []object.TreeEntry{
		{
			Name: "test-file-read",
			Mode: filemode.Regular,
			Hash: readHash,
		},
		{
			Name: "test-file-write",
			Mode: filemode.Regular,
			Hash: writeHash,
		},
	}

	treeHash, err := WriteTree(repo, entries)
	if err != nil {
		t.Error(err)
	}

	tree, err := GetTree(repo, treeHash)
	if err != nil {
		t.Error(err)
	}

	assert.Equal(t, "e8df153fd5749966e7ddf148fcbee17d747753ae", treeHash.String())
	assert.Equal(t, entries, tree.Entries)
}

func TestEmptyTree(t *testing.T) {
	hash := EmptyTree()

	// SHA-1 ID used by Git to denote an empty tree
	// $ git hash-object -t tree --stdin < /dev/null
	assert.Equal(t, "4b825dc642cb6eb9a060e54bf8d69288fbee4904", hash.String())
}

func TestTreeBuilder(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	blobA, err := WriteBlob(repo, []byte("a"))
	if err != nil {
		t.Fatal(err)
	}
	blobB, err := WriteBlob(repo, []byte("b"))
	if err != nil {
		t.Fatal(err)
	}

	t.Run("no blobs", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)
		_, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
		assert.ErrorIs(t, err, ErrNoEntries)

		_, err = treeBuilder.WriteRootTreeFromBlobIDs(map[string]plumbing.Hash{})
		assert.ErrorIs(t, err, ErrNoEntries)
	})

	t.Run("both blobs in the root directory", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := map[string]plumbing.Hash{
			"a": blobA,
			"b": blobB,
		}

		rootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(input)
		assert.Nil(t, err)

		tree, err := repo.TreeObject(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		// Assert number of entries
		assert.Equal(t, 2, len(tree.Entries))

		// Find entry "a"
		entryA, err := tree.FindEntry("a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, blobA, entryA.Hash)

		// Find entry "b"
		entryB, err := tree.FindEntry("b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, blobB, entryB.Hash)
	})

	t.Run("both blobs in same subdirectory", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := map[string]plumbing.Hash{
			"dir/a": blobA,
			"dir/b": blobB,
		}

		rootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(input)
		assert.Nil(t, err)

		tree, err := repo.TreeObject(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		// Assert number of entries, and that it's the subdirectory
		assert.Equal(t, 1, len(tree.Entries))
		assert.Equal(t, filemode.Dir, tree.Entries[0].Mode)

		// Find entry "a"
		entryA, err := tree.FindEntry("dir/a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, blobA, entryA.Hash)

		// Find entry "b"
		entryB, err := tree.FindEntry("dir/b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, blobB, entryB.Hash)
	})

	t.Run("both blobs in different subdirectories", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := map[string]plumbing.Hash{
			"foo/a": blobA,
			"bar/b": blobB,
		}

		rootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(input)
		assert.Nil(t, err)

		tree, err := repo.TreeObject(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		// Assert number of entries, and that it's the two subdirectories
		assert.Equal(t, 2, len(tree.Entries))
		assert.Equal(t, filemode.Dir, tree.Entries[0].Mode)
		assert.Equal(t, filemode.Dir, tree.Entries[1].Mode)

		// Find entry "a"
		entryA, err := tree.FindEntry("foo/a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, blobA, entryA.Hash)

		// Find entry "b"
		entryB, err := tree.FindEntry("bar/b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, blobB, entryB.Hash)
	})

	t.Run("blobs in mix of root directory and subdirectories", func(t *testing.T) {
		treeBuilder := NewTreeBuilder(repo)

		input := map[string]plumbing.Hash{
			"a":                blobA,
			"foo/bar/foobar/b": blobB,
		}

		rootTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(input)
		assert.Nil(t, err)

		tree, err := repo.TreeObject(rootTreeID)
		if err != nil {
			t.Fatal(err)
		}

		// Assert number of entries, and that it's one file and one directory
		assert.Equal(t, 2, len(tree.Entries))
		assert.Equal(t, filemode.Regular, tree.Entries[0].Mode)
		assert.Equal(t, filemode.Dir, tree.Entries[1].Mode)

		// Find entry "a"
		entryA, err := tree.FindEntry("a")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, blobA, entryA.Hash)

		// Find entry "b"
		entryB, err := tree.FindEntry("foo/bar/foobar/b")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, blobB, entryB.Hash)
	})
}
