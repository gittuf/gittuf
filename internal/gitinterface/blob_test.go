// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"io"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestReadBlob(t *testing.T) {
	readContents := []byte("test file read")

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	testObj := repo.Storer.NewEncodedObject()
	testObj.SetType(plumbing.BlobObject)

	writer, err := testObj.Writer()
	if err != nil {
		t.Fatal(err)
	}

	length, err := writer.Write(readContents)
	if err != nil {
		t.Fatal(err)
	} else if length != len(readContents) {
		t.Fatal(fmt.Errorf("unable to write all of test contents"))
	}

	writtenHash, err := repo.Storer.SetEncodedObject(testObj)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("test expected file", func(t *testing.T) {
		expectedHash := "2ecdd330475d93568ed27f717a84a7fe207d1c58"

		contents, err := ReadBlob(repo, plumbing.NewHash(expectedHash))
		if err != nil {
			t.Error(err)
		}

		assert.Equal(t, expectedHash, writtenHash.String())
		assert.Equal(t, readContents, contents)
	})

	t.Run("test nonexistent blob", func(t *testing.T) {
		_, err := ReadBlob(repo, plumbing.ZeroHash)
		assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)
	})

	t.Run("test non blob", func(t *testing.T) {
		treeHash, err := WriteTree(repo, []object.TreeEntry{
			{
				Name: "blob",
				Mode: filemode.Regular,
				Hash: writtenHash,
			},
		})
		if err != nil {
			t.Fatal(err)
		}

		_, err = ReadBlob(repo, treeHash)
		assert.ErrorIs(t, err, plumbing.ErrObjectNotFound)
	})
}

func TestRepositoryReadBlob(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir)

	contents := []byte("test file read")
	expectedBlobID := Hash{hash: "2ecdd330475d93568ed27f717a84a7fe207d1c58"}

	blobID, err := repo.WriteBlob(contents)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedBlobID, blobID)

	t.Run("read existing blob", func(t *testing.T) {
		readContents, err := repo.ReadBlob(blobID)
		assert.Nil(t, err)
		assert.Equal(t, contents, readContents)
	})

	t.Run("read non-existing blob", func(t *testing.T) {
		_, err := repo.ReadBlob(ZeroHash)
		assert.NotNil(t, err)
	})
}

func TestWriteBlob(t *testing.T) {
	writeContents := []byte("test file write")

	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	blobID, err := WriteBlob(repo, writeContents)
	if err != nil {
		t.Error(err)
	}

	expectedHash := plumbing.NewHash("999c05e9578e5d244920306842f516789a2498f7")
	assert.Equal(t, expectedHash, blobID)

	obj, err := GetBlob(repo, blobID)
	if err != nil {
		t.Fatal(err)
	}

	reader, err := obj.Reader()
	if err != nil {
		t.Fatal(err)
	}

	writtenContents, err := io.ReadAll(reader)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, writeContents, writtenContents)
}

func TestRepositoryWriteBlob(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir)

	contents := []byte("test file write")
	expectedBlobID := Hash{hash: "999c05e9578e5d244920306842f516789a2498f7"}

	blobID, err := repo.WriteBlob(contents)
	assert.Nil(t, err)
	assert.Equal(t, expectedBlobID, blobID)
}

func TestEmptyBlob(t *testing.T) {
	hash := EmptyBlob()

	// SHA-1 ID used by Git to denote an empty blob
	// $ git hash-object -t blob --stdin < /dev/null
	assert.Equal(t, "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391", hash.String())
}

func TestRepositoryEmptyBlob(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir)

	hash, err := repo.EmptyBlob()
	assert.Nil(t, err)

	// SHA-1 ID used by Git to denote an empty blob
	// $ git hash-object -t tree --blob < /dev/null
	assert.Equal(t, Hash{hash: "e69de29bb2d1d6434b8b29ae775ad8c2e48c5391"}, hash)
}
