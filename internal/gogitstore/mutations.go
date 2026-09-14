// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gogitstore

import (
	"time"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

// Mutations invalidate the cached handle so subsequent reads see new packfiles.

func (s *Storer) WriteBlob(contents []byte) (githash.Hash, error) {
	defer s.record("WriteBlob", time.Now(), true)
	defer s.invalidate()
	return s.Repository.WriteBlob(contents)
}

func (s *Storer) WriteTree(entries []gitstore.TreeEntry) (githash.Hash, error) {
	defer s.record("WriteTree", time.Now(), true)
	defer s.invalidate()
	return s.Repository.WriteTree(entries)
}

func (s *Storer) Commit(treeID githash.Hash, targetRef, message string, sign bool) (githash.Hash, error) {
	defer s.record("Commit", time.Now(), true)
	defer s.invalidate()
	return s.Repository.Commit(treeID, targetRef, message, sign)
}

func (s *Storer) CommitUsingSpecificKey(treeID githash.Hash, targetRef, message string, signingKeyPEMBytes []byte) (githash.Hash, error) {
	defer s.record("CommitUsingSpecificKey", time.Now(), true)
	defer s.invalidate()
	return s.Repository.CommitUsingSpecificKey(treeID, targetRef, message, signingKeyPEMBytes)
}

func (s *Storer) TagUsingSpecificKey(target githash.Hash, name, message string, signingKeyPEMBytes []byte) (githash.Hash, error) {
	defer s.record("TagUsingSpecificKey", time.Now(), true)
	defer s.invalidate()
	return s.Repository.TagUsingSpecificKey(target, name, message, signingKeyPEMBytes)
}

func (s *Storer) SetReference(refName string, gitID githash.Hash) error {
	defer s.record("SetReference", time.Now(), true)
	defer s.invalidate()
	return s.Repository.SetReference(refName, gitID)
}

func (s *Storer) DeleteReference(refName string) error {
	defer s.record("DeleteReference", time.Now(), true)
	defer s.invalidate()
	return s.Repository.DeleteReference(refName)
}

func (s *Storer) ResetDueToError(cause error, refName string, commitID githash.Hash) error {
	defer s.record("ResetDueToError", time.Now(), true)
	defer s.invalidate()
	return s.Repository.ResetDueToError(cause, refName, commitID)
}

func (s *Storer) Fetch(remoteName string, refs []string, fastForwardOnly bool, opts ...gitinterface.FetchOption) error {
	defer s.record("Fetch", time.Now(), true)
	defer s.invalidate()
	return s.Repository.Fetch(remoteName, refs, fastForwardOnly, opts...)
}

func (s *Storer) FetchRefSpec(remoteName string, refSpecs []string, opts ...gitinterface.FetchOption) error {
	defer s.record("FetchRefSpec", time.Now(), true)
	defer s.invalidate()
	return s.Repository.FetchRefSpec(remoteName, refSpecs, opts...)
}

func (s *Storer) FetchObject(remoteName string, objectID githash.Hash) error {
	defer s.record("FetchObject", time.Now(), true)
	defer s.invalidate()
	return s.Repository.FetchObject(remoteName, objectID)
}

func (s *Storer) Push(remoteName string, refs []string) error {
	defer s.record("Push", time.Now(), true)
	defer s.invalidate()
	return s.Repository.Push(remoteName, refs)
}
