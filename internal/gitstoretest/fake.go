// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Package gitstoretest provides shared helpers for exercising gitstore.Storer
// error paths in tests.
package gitstoretest

import (
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

// FakeStorer wraps a real gitstore.Storer, injecting failures or canned
// values for the methods a test needs to control while delegating everything
// else to the embedded Storer. A test sets only the fields it cares about.
type FakeStorer struct {
	gitstore.Storer

	EmptyTreeErr          error
	GetReferenceErr       error
	GetCommitInfoErr      error
	GetAllFilesInTreeErr  error
	WriteTreeErr          error
	ReadBlobErr           error
	WriteBlobErr          error
	CommitErr             error
	GetObjectSignatureErr error

	// Config supplies canned config values keyed by ConfigKey. A key present
	// here is returned by LookupConfig with ok true. A key absent here falls
	// through to the embedded Storer when it is non-nil, otherwise LookupConfig
	// reports ok false. LookupConfigErr takes precedence over both.
	Config          map[gitstore.ConfigKey]string
	LookupConfigErr error
}

func (f *FakeStorer) EmptyTree() (githash.Hash, error) {
	if f.EmptyTreeErr != nil {
		return nil, f.EmptyTreeErr
	}
	return f.Storer.EmptyTree()
}

func (f *FakeStorer) GetReference(refName string) (githash.Hash, error) {
	if f.GetReferenceErr != nil {
		return nil, f.GetReferenceErr
	}
	return f.Storer.GetReference(refName)
}

func (f *FakeStorer) GetCommitInfo(commitID githash.Hash) (gitstore.CommitInfo, error) {
	if f.GetCommitInfoErr != nil {
		return gitstore.CommitInfo{}, f.GetCommitInfoErr
	}
	return f.Storer.GetCommitInfo(commitID)
}

func (f *FakeStorer) GetAllFilesInTree(treeID githash.Hash) (map[string]githash.Hash, error) {
	if f.GetAllFilesInTreeErr != nil {
		return nil, f.GetAllFilesInTreeErr
	}
	return f.Storer.GetAllFilesInTree(treeID)
}

func (f *FakeStorer) WriteTree(entries []gitstore.TreeEntry) (githash.Hash, error) {
	if f.WriteTreeErr != nil {
		return nil, f.WriteTreeErr
	}
	return f.Storer.WriteTree(entries)
}

func (f *FakeStorer) ReadBlob(blobID githash.Hash) ([]byte, error) {
	if f.ReadBlobErr != nil {
		return nil, f.ReadBlobErr
	}
	return f.Storer.ReadBlob(blobID)
}

func (f *FakeStorer) WriteBlob(contents []byte) (githash.Hash, error) {
	if f.WriteBlobErr != nil {
		return nil, f.WriteBlobErr
	}
	return f.Storer.WriteBlob(contents)
}

func (f *FakeStorer) Commit(treeID githash.Hash, targetRef, message string, sign bool) (githash.Hash, error) {
	if f.CommitErr != nil {
		return nil, f.CommitErr
	}
	return f.Storer.Commit(treeID, targetRef, message, sign)
}

func (f *FakeStorer) GetObjectSignature(objectID githash.Hash) ([]byte, []byte, error) {
	if f.GetObjectSignatureErr != nil {
		return nil, nil, f.GetObjectSignatureErr
	}
	return f.Storer.GetObjectSignature(objectID)
}

func (f *FakeStorer) LookupConfig(key gitstore.ConfigKey) (string, bool, error) {
	if f.LookupConfigErr != nil {
		return "", false, f.LookupConfigErr
	}
	if value, ok := f.Config[key]; ok {
		return value, true, nil
	}
	if f.Storer != nil {
		return f.Storer.LookupConfig(key)
	}
	return "", false, nil
}
