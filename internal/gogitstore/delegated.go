// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gogitstore

import (
	"time"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

// These overrides trace operations delegated to Git.

func (s *Storer) EmptyTree() (githash.Hash, error) {
	defer s.record("EmptyTree", time.Now(), true)
	return s.Repository.EmptyTree()
}

func (s *Storer) LookupConfig(key gitstore.ConfigKey) (string, bool, error) {
	defer s.record("LookupConfig", time.Now(), true)
	return s.Repository.LookupConfig(key)
}

func (s *Storer) GetCommitsBetweenRange(commitNewID, commitOldID githash.Hash) ([]githash.Hash, error) {
	defer s.record("GetCommitsBetweenRange", time.Now(), true)
	return s.Repository.GetCommitsBetweenRange(commitNewID, commitOldID)
}

func (s *Storer) GetFilePathsChangedByCommit(commitID githash.Hash) ([]string, error) {
	defer s.record("GetFilePathsChangedByCommit", time.Now(), true)
	return s.Repository.GetFilePathsChangedByCommit(commitID)
}

func (s *Storer) GetMergeTree(commitAID, commitBID githash.Hash) (githash.Hash, error) {
	defer s.record("GetMergeTree", time.Now(), true)
	return s.Repository.GetMergeTree(commitAID, commitBID)
}

// KnowsCommit delegates to Git to respect shallow-history boundaries.
func (s *Storer) KnowsCommit(testCommitID, ancestorCommitID githash.Hash) (bool, error) {
	defer s.record("KnowsCommit", time.Now(), true)
	return s.Repository.KnowsCommit(testCommitID, ancestorCommitID)
}
