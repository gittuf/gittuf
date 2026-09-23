// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gogitstore_test

import (
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/gogitstore"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFetchInvalidatesCachedHandle(t *testing.T) {
	t.Parallel()
	for _, format := range objectFormats {
		t.Run(string(format), func(t *testing.T) {
			t.Parallel()
			for name, fetch := range map[string]func(*gogitstore.Storer, githash.Hash) error{
				"Fetch": func(s *gogitstore.Storer, _ githash.Hash) error {
					return s.Fetch("origin", []string{"refs/gittuf/remote-only"}, true)
				},
				"FetchRefSpec": func(s *gogitstore.Storer, _ githash.Hash) error {
					return s.FetchRefSpec("origin", []string{"refs/gittuf/remote-only:refs/gittuf/remote-only"})
				},
				"FetchObject": func(s *gogitstore.Storer, id githash.Hash) error { return s.FetchObject("origin", id) },
			} {
				t.Run(name, func(t *testing.T) {
					t.Parallel()
					remoteDir := t.TempDir()
					remote := gitinterface.CreateTestGitRepository(t, remoteDir, true, gitinterface.WithObjectFormat(format))
					tree, err := remote.EmptyTree()
					require.NoError(t, err)
					remoteID, err := remote.Commit(tree, "refs/gittuf/remote-only", "remote", false)
					require.NoError(t, err)
					local, fast := newBackends(t, format)
					// Keep fetched objects packed to exercise the cached pack index.
					git(t, local, "", "config", "fetch.unpackLimit", "1")
					localID := commitChain(t, local, "refs/heads/local", 2)[1]
					_, err = fast.GetCommitMessage(localID)
					require.NoError(t, err)
					require.NoError(t, fast.AddRemote("origin", remoteDir))
					require.NoError(t, fetch(fast, remoteID))
					packs, err := filepath.Glob(filepath.Join(local.GetGitDir(), "objects", "pack", "*.pack"))
					require.NoError(t, err)
					require.NotEmpty(t, packs)
					message, err := fast.GetCommitMessage(remoteID)
					require.NoError(t, err)
					assert.Equal(t, "remote", message)
				})
			}
		})
	}
}

func TestWriteThenReadThroughWrapper(t *testing.T) {
	t.Parallel()

	local := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	fast := gogitstore.New(local, true)

	existing := commitChain(t, local, "refs/heads/main", 1)
	_, err := fast.GetCommitMessage(existing[0])
	require.Nil(t, err)

	blobID, err := fast.WriteBlob([]byte("written after the handle was opened"))
	require.Nil(t, err)

	contents, err := fast.ReadBlob(blobID)
	require.Nil(t, err)
	assert.Equal(t, []byte("written after the handle was opened"), contents)
}
