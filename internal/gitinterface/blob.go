package gitinterface

import (
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
)

func ReadBlob(repo *git.Repository, blobID plumbing.Hash) (int, []byte, error) {
	blob, err := repo.BlobObject(blobID)
	if err != nil {
		return -1, []byte{}, err
	}
	contents := make([]byte, blob.Size)
	reader, err := blob.Reader()
	if err != nil {
		return -1, []byte{}, err
	}
	length, err := reader.Read(contents)
	if err != nil {
		return -1, []byte{}, err
	}
	return length, contents, nil
}

func WriteBlob(repo *git.Repository, contents []byte) (int, plumbing.Hash, error) {
	obj := repo.Storer.NewEncodedObject()
	obj.SetType(plumbing.BlobObject)
	writer, err := obj.Writer()
	if err != nil {
		return -1, plumbing.ZeroHash, err
	}
	length, err := writer.Write(contents)
	if err != nil {
		return length, plumbing.ZeroHash, err
	}
	blobID, err := repo.Storer.SetEncodedObject(obj)
	return length, blobID, err
}
