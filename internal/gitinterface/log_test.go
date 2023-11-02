// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"fmt"
	"sort"
	"testing"

	"github.com/gittuf/gittuf/internal/third_party/go-git"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing"
	"github.com/gittuf/gittuf/internal/third_party/go-git/plumbing/object"
	"github.com/gittuf/gittuf/internal/third_party/go-git/storage/memory"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/stretchr/testify/assert"
)

func TestGetCommitsBetweenRange(t *testing.T) {
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	refName := plumbing.ReferenceName("refs/heads/main")
	if err := repo.Storer.SetReference(plumbing.NewHashReference(refName, plumbing.ZeroHash)); err != nil {
		t.Fatal(err)
	}

	ref, err := repo.Reference(refName, true)
	if err != nil {
		t.Fatal(err)
	}

	emptyBlobHash, err := WriteBlob(repo, []byte{})
	if err != nil {
		t.Fatal(err)
	}

	treeHashes := make([]plumbing.Hash, 0, 5)
	for i := 1; i <= 5; i++ {
		objects := make([]object.TreeEntry, 0, i)
		for j := 0; j < i; j++ {
			objects = append(objects, object.TreeEntry{Name: fmt.Sprintf("%d", j+1), Hash: emptyBlobHash})
		}

		treeHash, err := WriteTree(repo, objects)
		if err != nil {
			t.Fatal(err)
		}

		treeHashes = append(treeHashes, treeHash)
	}

	commitIDs := []plumbing.Hash{}
	for i := 0; i < 5; i++ {
		commit := CreateCommitObject(testGitConfig, treeHashes[i], ref.Hash(), "Test commit", testClock)
		if _, err := ApplyCommit(repo, commit, ref); err != nil {
			t.Fatal(err)
		}

		ref, err = repo.Reference(refName, true)
		if err != nil {
			t.Fatal(err)
		}

		commitIDs = append(commitIDs, ref.Hash())
	}

	allCommits := make([]*object.Commit, 0, len(commitIDs))
	for _, commitID := range commitIDs {
		commit, err := GetCommit(repo, commitID)
		if err != nil {
			t.Fatal(err)
		}

		allCommits = append(allCommits, commit)
	}

	t.Run("Check range between commits 1 and 5", func(t *testing.T) {
		commits, err := GetCommitsBetweenRange(repo, commitIDs[4], commitIDs[0])
		assert.Nil(t, err)
		expectedCommits := []*object.Commit{allCommits[4], allCommits[3], allCommits[2], allCommits[1]}
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].ID().String() < expectedCommits[j].ID().String()
		})
		assert.Equal(t, expectedCommits, commits)
	})

	t.Run("Pass in wrong order", func(t *testing.T) {
		// TODO: is this expected behavior?
		commits, err := GetCommitsBetweenRange(repo, commitIDs[0], commitIDs[4])
		assert.Nil(t, err)
		assert.Empty(t, commits)
	})

	t.Run("Get all commits", func(t *testing.T) {
		commits, err := GetCommitsBetweenRange(repo, commitIDs[4], plumbing.ZeroHash)
		assert.Nil(t, err)
		expectedCommits := allCommits
		sort.Slice(expectedCommits, func(i, j int) bool {
			return expectedCommits[i].ID().String() < expectedCommits[j].ID().String()
		})
		assert.Equal(t, expectedCommits, commits)
	})
}
