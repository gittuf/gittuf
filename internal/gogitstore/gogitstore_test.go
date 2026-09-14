// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gogitstore_test

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/gogitstore"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var objectFormats = []gitinterface.ObjectFormat{
	gitinterface.ObjectFormatSHA1,
	gitinterface.ObjectFormatSHA256,
}

// newBackends returns both backends over the same repository.
func newBackends(t *testing.T, objectFormat gitinterface.ObjectFormat) (*gitinterface.Repository, *gogitstore.Storer) {
	t.Helper()

	binary := gitinterface.CreateTestGitRepository(t, t.TempDir(), false, gitinterface.WithObjectFormat(objectFormat))
	return binary, gogitstore.New(binary, true)
}

func TestReadBlob(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)

			blobID, err := binary.WriteBlob([]byte("hello there"))
			require.Nil(t, err)

			want, err := binary.ReadBlob(blobID)
			require.Nil(t, err)

			got, err := fast.ReadBlob(blobID)
			require.Nil(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func commitChain(t *testing.T, repo *gitinterface.Repository, refName string, n int) []githash.Hash {
	t.Helper()

	treeID, err := repo.EmptyTree()
	require.Nil(t, err)

	ids := make([]githash.Hash, 0, n)
	for i := range n {
		id, err := repo.Commit(treeID, refName, fmt.Sprintf("commit %d\n", i), false)
		require.Nil(t, err)
		ids = append(ids, id)
	}

	return ids
}

func TestGetCommitMessage(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)
			ids := commitChain(t, binary, "refs/heads/main", 1)

			want, err := binary.GetCommitMessage(ids[0])
			require.Nil(t, err)

			got, err := fast.GetCommitMessage(ids[0])
			require.Nil(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestGetCommitParentIDs(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)
			ids := commitChain(t, binary, "refs/heads/main", 2)

			t.Run("commit with a parent", func(t *testing.T) {
				want, err := binary.GetCommitParentIDs(ids[1])
				require.Nil(t, err)

				got, err := fast.GetCommitParentIDs(ids[1])
				require.Nil(t, err)
				assert.Equal(t, want, got)
			})

			t.Run("initial commit has no parents", func(t *testing.T) {
				want, err := binary.GetCommitParentIDs(ids[0])
				require.Nil(t, err)

				got, err := fast.GetCommitParentIDs(ids[0])
				require.Nil(t, err)
				assert.Equal(t, want, got)
			})
		})
	}
}

func TestGetCommitTreeID(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)
			ids := commitChain(t, binary, "refs/heads/main", 1)

			want, err := binary.GetCommitTreeID(ids[0])
			require.Nil(t, err)

			got, err := fast.GetCommitTreeID(ids[0])
			require.Nil(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestKnowsCommit(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)
			ids := commitChain(t, binary, "refs/heads/main", 3)

			t.Run("ancestor", func(t *testing.T) {
				want, err := binary.KnowsCommit(ids[2], ids[0])
				require.Nil(t, err)
				require.True(t, want)

				got, err := fast.KnowsCommit(ids[2], ids[0])
				require.Nil(t, err)
				assert.Equal(t, want, got)
			})

			t.Run("not an ancestor", func(t *testing.T) {
				want, err := binary.KnowsCommit(ids[0], ids[2])
				require.Nil(t, err)
				require.False(t, want)

				got, err := fast.KnowsCommit(ids[0], ids[2])
				require.Nil(t, err)
				assert.Equal(t, want, got)
			})

			t.Run("commit knows itself", func(t *testing.T) {
				want, err := binary.KnowsCommit(ids[1], ids[1])
				require.Nil(t, err)

				got, err := fast.KnowsCommit(ids[1], ids[1])
				require.Nil(t, err)
				assert.Equal(t, want, got)
			})
		})
	}
}

func TestShallowHistory(t *testing.T) {
	t.Parallel()
	for _, format := range objectFormats {
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()
			binary, fast := newBackends(t, format)
			ids := commitChain(t, binary, "refs/heads/main", 3)
			require.NoError(t, os.WriteFile(filepath.Join(binary.GetGitDir(), "shallow"), []byte(ids[1].String()+"\n"), 0o600))
			for _, pair := range [][2]int{{2, 1}, {2, 0}, {1, 0}, {1, 1}} {
				want, err := binary.KnowsCommit(ids[pair[0]], ids[pair[1]])
				require.NoError(t, err)
				got, err := fast.KnowsCommit(ids[pair[0]], ids[pair[1]])
				require.NoError(t, err)
				assert.Equal(t, want, got)
			}
			for _, id := range ids {
				want, err := binary.GetCommitParentIDs(id)
				require.NoError(t, err)
				got, err := fast.GetCommitParentIDs(id)
				require.NoError(t, err)
				assert.Equal(t, want, got)
			}
		})
	}
}
