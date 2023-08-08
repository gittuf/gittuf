package gitinterface

import (
	"fmt"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	format "github.com/go-git/go-git/v5/plumbing/format/config"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/jonboulle/clockwork"
	"github.com/stretchr/testify/assert"
)

func TestGetCommitFilePaths(t *testing.T) {
	gitConfig := &config.Config{
		Raw: &format.Config{
			Sections: format.Sections{
				&format.Section{
					Name: "user",
					Options: format.Options{
						&format.Option{
							Key:   "name",
							Value: "Jane Doe",
						},
						&format.Option{
							Key:   "email",
							Value: "jane.doe@example.com",
						},
					},
				},
			},
		},
	}

	clock := clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
	repo, err := git.Init(memory.NewStorage(), memfs.New())
	if err != nil {
		t.Fatal(err)
	}

	emptyBlobHash := EmptyBlob()

	tests := map[string]struct {
		treeEntries   []object.TreeEntry
		expectedPaths []string
	}{
		"one file": {
			treeEntries: []object.TreeEntry{
				{
					Name: "a",
					Mode: filemode.Regular,
					Hash: emptyBlobHash,
				},
			},
			expectedPaths: []string{"a"},
		},
		"multiple files": {
			treeEntries: []object.TreeEntry{
				{
					Name: "a",
					Mode: filemode.Regular,
					Hash: emptyBlobHash,
				},
				{
					Name: "b",
					Mode: filemode.Regular,
					Hash: emptyBlobHash,
				},
			},
			expectedPaths: []string{"a", "b"},
		},
		"no files": {
			treeEntries:   []object.TreeEntry{},
			expectedPaths: []string{},
		},
	}

	for name, test := range tests {
		WriteBlob(repo, []byte{}) //nolint: errcheck
		treeHash, err := WriteTree(repo, test.treeEntries)
		if err != nil {
			t.Fatal(err)
		}

		c := CreateCommitObject(gitConfig, treeHash, plumbing.ZeroHash, "Test commit", clock)
		commitID, err := WriteCommit(repo, c)
		if err != nil {
			t.Fatal(err)
		}
		commit, err := repo.CommitObject(commitID)
		if err != nil {
			t.Fatal(err)
		}

		paths, err := GetCommitFilePaths(repo, commit)
		assert.Nil(t, err, fmt.Sprintf("unexpected error in test %s", name))
		assert.Equal(t, test.expectedPaths, paths, fmt.Sprintf("unexpected list of files received: expected %v, got %v in test %s", test.expectedPaths, paths, name))
	}
}
