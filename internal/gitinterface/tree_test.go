// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/gittuf/gittuf/internal/third_party/go-git"
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
