// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"testing"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestRefSpec(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	shortRefName := "master"
	qualifiedRefName := "refs/heads/master"
	qualifiedRemoteRefName := "refs/remotes/origin/master"
	emptyTreeHash, err := WriteTree(repo, nil)
	if err != nil {
		t.Fatal(err)
	}
	commitID, err := Commit(repo, emptyTreeHash, qualifiedRefName, "Test Commit", false)
	if err != nil {
		t.Fatal(err)
	}
	refHash, err := repo.ResolveRevision(plumbing.Revision(qualifiedRefName))
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, commitID, *refHash, "unexpected value configuring test repo")

	tests := map[string]struct {
		repo            *git.Repository
		refName         string
		remoteName      string
		fastForwardOnly bool
		expectedRefSpec config.RefSpec
		expectedError   error
	}{
		"standard branch, not fast forward only, no remote": {
			refName:         "refs/heads/main",
			expectedRefSpec: config.RefSpec("+refs/heads/main:refs/heads/main"),
		},
		"standard branch, fast forward only, no remote": {
			refName:         "refs/heads/main",
			fastForwardOnly: true,
			expectedRefSpec: config.RefSpec("refs/heads/main:refs/heads/main"),
		},
		"standard branch, not fast forward only, remote": {
			refName:         "refs/heads/main",
			remoteName:      "origin",
			expectedRefSpec: config.RefSpec("+refs/heads/main:refs/remotes/origin/main"),
		},
		"standard branch, fast forward only, remote": {
			refName:         "refs/heads/main",
			remoteName:      "origin",
			fastForwardOnly: true,
			expectedRefSpec: config.RefSpec("refs/heads/main:refs/remotes/origin/main"),
		},
		"non-standard branch, not fast forward only, no remote": {
			refName:         "refs/heads/foo/bar",
			expectedRefSpec: config.RefSpec("+refs/heads/foo/bar:refs/heads/foo/bar"),
		},
		"non-standard branch, fast forward only, no remote": {
			refName:         "refs/heads/foo/bar",
			fastForwardOnly: true,
			expectedRefSpec: config.RefSpec("refs/heads/foo/bar:refs/heads/foo/bar"),
		},
		"non-standard branch, not fast forward only, remote": {
			refName:         "refs/heads/foo/bar",
			remoteName:      "origin",
			expectedRefSpec: config.RefSpec("+refs/heads/foo/bar:refs/remotes/origin/foo/bar"),
		},
		"non-standard branch, fast forward only, remote": {
			refName:         "refs/heads/foo/bar",
			remoteName:      "origin",
			fastForwardOnly: true,
			expectedRefSpec: config.RefSpec("refs/heads/foo/bar:refs/remotes/origin/foo/bar"),
		},
		"short branch, not fast forward only, no remote": {
			refName:         shortRefName,
			repo:            repo,
			expectedRefSpec: config.RefSpec(fmt.Sprintf("+%s:%s", qualifiedRefName, qualifiedRefName)),
		},
		"short branch, fast forward only, no remote": {
			refName:         shortRefName,
			repo:            repo,
			fastForwardOnly: true,
			expectedRefSpec: config.RefSpec(fmt.Sprintf("%s:%s", qualifiedRefName, qualifiedRefName)),
		},
		"short branch, not fast forward only, remote": {
			refName:         shortRefName,
			repo:            repo,
			remoteName:      "origin",
			expectedRefSpec: config.RefSpec(fmt.Sprintf("+%s:%s", qualifiedRefName, qualifiedRemoteRefName)),
		},
		"short branch, fast forward only, remote": {
			refName:         shortRefName,
			repo:            repo,
			fastForwardOnly: true,
			remoteName:      "origin",
			expectedRefSpec: config.RefSpec(fmt.Sprintf("%s:%s", qualifiedRefName, qualifiedRemoteRefName)),
		},
		"custom namespace, not fast forward only, no remote": {
			refName:         "refs/foo/bar",
			expectedRefSpec: config.RefSpec("+refs/foo/bar:refs/foo/bar"),
		},
		"custom namespace, fast forward only, no remote": {
			refName:         "refs/foo/bar",
			fastForwardOnly: true,
			expectedRefSpec: config.RefSpec("refs/foo/bar:refs/foo/bar"),
		},
		"custom namespace, not fast forward only, remote": {
			refName:         "refs/foo/bar",
			remoteName:      "origin",
			expectedRefSpec: config.RefSpec("+refs/foo/bar:refs/remotes/origin/foo/bar"),
		},
		"custom namespace, fast forward only, remote": {
			refName:         "refs/foo/bar",
			remoteName:      "origin",
			fastForwardOnly: true,
			expectedRefSpec: config.RefSpec("refs/foo/bar:refs/remotes/origin/foo/bar"),
		},
		"tag, not fast forward only, no remote": {
			refName:         "refs/tags/v1.0.0",
			fastForwardOnly: false,
			expectedRefSpec: config.RefSpec("refs/tags/v1.0.0:refs/tags/v1.0.0"),
		},
		"tag, fast forward only, no remote": {
			refName:         "refs/tags/v1.0.0",
			fastForwardOnly: true,
			expectedRefSpec: config.RefSpec("refs/tags/v1.0.0:refs/tags/v1.0.0"),
		},
		"tag, not fast forward only, remote": {
			refName:         "refs/tags/v1.0.0",
			remoteName:      "origin",
			fastForwardOnly: false,
			expectedRefSpec: config.RefSpec("refs/tags/v1.0.0:refs/tags/v1.0.0"),
		},
		"tag, fast forward only, remote": {
			refName:         "refs/tags/v1.0.0",
			remoteName:      "origin",
			fastForwardOnly: true,
			expectedRefSpec: config.RefSpec("refs/tags/v1.0.0:refs/tags/v1.0.0"),
		},
	}

	for name, test := range tests {
		refSpec, err := RefSpec(test.repo, test.refName, test.remoteName, test.fastForwardOnly)
		assert.ErrorIs(t, err, test.expectedError, fmt.Sprintf("unexpected error in test '%s'", name))
		assert.Equal(t, test.expectedRefSpec, refSpec, fmt.Sprintf("unexpected refspec returned in test '%s'", name))
	}
}
