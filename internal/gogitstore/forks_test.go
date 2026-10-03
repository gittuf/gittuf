// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gogitstore_test

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/gittuf/gittuf/internal/gogitstore"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// countGitForks puts a shim ahead of git on PATH and reports how many times it
// ran. PATH is process global, so callers must not use t.Parallel.
func countGitForks(t *testing.T) func() int {
	t.Helper()

	if runtime.GOOS == "windows" {
		t.Skip("the git shim used to count forks is a POSIX shell script")
	}

	realGit, err := exec.LookPath("git")
	require.Nil(t, err)

	shimDir := t.TempDir()
	logPath := filepath.Join(shimDir, "invocations")
	shim := fmt.Sprintf("#!/bin/sh\necho invoked >> %q\nexec %q \"$@\"\n", logPath, realGit)
	require.Nil(t, os.WriteFile(filepath.Join(shimDir, "git"), []byte(shim), 0o700)) //nolint:gosec // the shim stands in for the git binary, so it has to be executable

	t.Setenv("PATH", shimDir+string(os.PathListSeparator)+os.Getenv("PATH"))

	return func() int {
		contents, err := os.ReadFile(logPath)
		if os.IsNotExist(err) {
			return 0
		}
		require.Nil(t, err)
		return bytes.Count(contents, []byte("\n"))
	}
}

type readFixture struct {
	repo     *gitinterface.Repository
	blobID   githash.Hash
	treeID   githash.Hash
	commitID githash.Hash
	signedID githash.Hash
	tagID    githash.Hash
	refName  string
}

func newReadFixture(t *testing.T) *readFixture {
	t.Helper()

	repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)

	blobID, err := repo.WriteBlob([]byte("policy metadata"))
	require.Nil(t, err)

	treeID, err := repo.WriteTree([]gitstore.TreeEntry{
		{Path: "root.json", ID: blobID, Kind: gitstore.KindBlob},
		{Path: "nested/targets.json", ID: blobID, Kind: gitstore.KindBlob},
	})
	require.Nil(t, err)

	refName := "refs/heads/main"
	_, err = repo.Commit(treeID, refName, "first\n", false)
	require.Nil(t, err)
	commitID, err := repo.Commit(treeID, refName, "second\n", false)
	require.Nil(t, err)

	signedID, err := repo.Commit(treeID, "refs/heads/signed", "signed\n", true)
	require.Nil(t, err)

	tagID, err := repo.TagUsingSpecificKey(commitID, "v1", "release\n", artifacts.SSHRSAPrivate)
	require.Nil(t, err)

	return &readFixture{
		repo: repo, blobID: blobID, treeID: treeID, commitID: commitID,
		signedID: signedID, tagID: tagID,
		refName: refName,
	}
}

func TestReadsDoNotForkGit(t *testing.T) {
	f := newReadFixture(t)

	reads := map[string]func(s *gogitstore.Storer) error{
		"ReadBlob": func(s *gogitstore.Storer) error {
			_, err := s.ReadBlob(f.blobID)
			return err
		},
		"GetCommitMessage": func(s *gogitstore.Storer) error {
			_, err := s.GetCommitMessage(f.commitID)
			return err
		},
		"GetCommitParentIDs": func(s *gogitstore.Storer) error {
			_, err := s.GetCommitParentIDs(f.commitID)
			return err
		},
		"GetCommitTreeID": func(s *gogitstore.Storer) error {
			_, err := s.GetCommitTreeID(f.commitID)
			return err
		},
		"GetEntriesInTree": func(s *gogitstore.Storer) error {
			_, err := s.GetEntriesInTree(f.treeID)
			return err
		},
		"GetAllFilesInTree": func(s *gogitstore.Storer) error {
			_, err := s.GetAllFilesInTree(f.treeID)
			return err
		},
		"GetPathIDInTree": func(s *gogitstore.Storer) error {
			_, err := s.GetPathIDInTree(f.treeID, "root.json")
			return err
		},
		"GetReference": func(s *gogitstore.Storer) error {
			_, err := s.GetReference(f.refName)
			return err
		},
		"GetTagTarget": func(s *gogitstore.Storer) error {
			_, err := s.GetTagTarget(f.tagID)
			return err
		},
		"GetObjectSignature": func(s *gogitstore.Storer) error {
			_, _, err := s.GetObjectSignature(f.signedID)
			return err
		},
	}

	for name, read := range reads {
		t.Run(name, func(t *testing.T) {
			fast := gogitstore.New(f.repo, true)

			forks := countGitForks(t)
			require.Nil(t, read(fast))
			assert.Zero(t, forks(), "%s forked the git binary", name)
		})
	}
}
