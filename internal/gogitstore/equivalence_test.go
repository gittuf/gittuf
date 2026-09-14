// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gogitstore_test

import (
	"fmt"
	"os/exec"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/internal/gogitstore"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// git runs a command for fixtures gitinterface has no API for, such as a tree
// holding a submodule gitlink.
func git(t *testing.T, repo *gitinterface.Repository, stdin string, args ...string) string {
	t.Helper()

	cmd := exec.Command("git", append([]string{"--git-dir", repo.GetGitDir()}, args...)...) //nolint:gosec
	if stdin != "" {
		cmd.Stdin = strings.NewReader(stdin)
	}
	out, err := cmd.Output()
	require.Nil(t, err, "git %s", strings.Join(args, " "))

	return strings.TrimSpace(string(out))
}

func TestGetEntriesInTree(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)

			blobID, err := binary.WriteBlob([]byte("contents"))
			require.Nil(t, err)
			treeID, err := binary.WriteTree([]gitstore.TreeEntry{
				{Path: "root.json", ID: blobID, Kind: gitstore.KindBlob},
				{Path: "nested/targets.json", ID: blobID, Kind: gitstore.KindBlob},
			})
			require.Nil(t, err)

			t.Run("blobs and subtrees", func(t *testing.T) {
				want, err := binary.GetEntriesInTree(treeID)
				require.Nil(t, err)

				got, err := fast.GetEntriesInTree(treeID)
				require.Nil(t, err)
				assert.Equal(t, want, got)

				kinds := map[string]gitstore.EntryKind{}
				for _, entry := range got {
					kinds[entry.Path] = entry.Kind
				}
				assert.Equal(t, gitstore.KindBlob, kinds["root.json"])
				assert.Equal(t, gitstore.KindSubtree, kinds["nested"])
			})

			t.Run("empty tree yields nil, not an empty slice", func(t *testing.T) {
				emptyTree, err := binary.EmptyTree()
				require.Nil(t, err)

				want, err := binary.GetEntriesInTree(emptyTree)
				require.Nil(t, err)
				require.Nil(t, want)

				got, err := fast.GetEntriesInTree(emptyTree)
				require.Nil(t, err)
				assert.Nil(t, got)
			})

			t.Run("submodule gitlink is reported as a blob", func(t *testing.T) {
				commitID, err := binary.Commit(treeID, "refs/heads/sub", "sub\n", false)
				require.Nil(t, err)

				gitlinkTree := git(t, binary,
					fmt.Sprintf("160000 commit %s\tsubmodule\n", commitID.String()),
					"mktree")
				gitlinkTreeID, err := githash.NewHash(gitlinkTree)
				require.Nil(t, err)

				want, err := binary.GetEntriesInTree(gitlinkTreeID)
				require.Nil(t, err)
				require.Len(t, want, 1)
				require.Equal(t, gitstore.KindBlob, want[0].Kind)

				got, err := fast.GetEntriesInTree(gitlinkTreeID)
				require.Nil(t, err)
				assert.Equal(t, want, got)
			})
		})
	}
}

func TestGetAllFilesInTree(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)

			blobID, err := binary.WriteBlob([]byte("contents"))
			require.Nil(t, err)
			treeID, err := binary.WriteTree([]gitstore.TreeEntry{
				{Path: "root.json", ID: blobID, Kind: gitstore.KindBlob},
				{Path: "a/b/c/deep.json", ID: blobID, Kind: gitstore.KindBlob},
			})
			require.Nil(t, err)

			t.Run("flattens nested trees", func(t *testing.T) {
				want, err := binary.GetAllFilesInTree(treeID)
				require.Nil(t, err)

				got, err := fast.GetAllFilesInTree(treeID)
				require.Nil(t, err)
				assert.Equal(t, want, got)
				assert.Contains(t, got, "a/b/c/deep.json")
			})

			t.Run("empty tree yields nil, not an empty map", func(t *testing.T) {
				emptyTree, err := binary.EmptyTree()
				require.Nil(t, err)

				want, err := binary.GetAllFilesInTree(emptyTree)
				require.Nil(t, err)
				require.Nil(t, want)

				got, err := fast.GetAllFilesInTree(emptyTree)
				require.Nil(t, err)
				assert.Nil(t, got)
			})

			t.Run("submodule gitlink is included as a leaf", func(t *testing.T) {
				commitID, err := binary.Commit(treeID, "refs/heads/sub2", "sub\n", false)
				require.Nil(t, err)

				gitlinkTree := git(t, binary,
					fmt.Sprintf("160000 commit %s\tsubmodule\n", commitID.String()),
					"mktree")
				gitlinkTreeID, err := githash.NewHash(gitlinkTree)
				require.Nil(t, err)

				want, err := binary.GetAllFilesInTree(gitlinkTreeID)
				require.Nil(t, err)
				require.Contains(t, want, "submodule")

				got, err := fast.GetAllFilesInTree(gitlinkTreeID)
				require.Nil(t, err)
				assert.Equal(t, want, got)
			})
		})
	}
}

func TestGetPathIDInTree(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)

			blobID, err := binary.WriteBlob([]byte("contents"))
			require.Nil(t, err)
			treeID, err := binary.WriteTree([]gitstore.TreeEntry{
				{Path: "root.json", ID: blobID, Kind: gitstore.KindBlob},
				{Path: "a/b/deep.json", ID: blobID, Kind: gitstore.KindBlob},
			})
			require.Nil(t, err)

			for _, treePath := range []string{"root.json", "a", "a/", "a/b", "a/b/deep.json"} {
				t.Run(treePath, func(t *testing.T) {
					want, err := binary.GetPathIDInTree(treeID, treePath)
					require.Nil(t, err)

					got, err := fast.GetPathIDInTree(treeID, treePath)
					require.Nil(t, err)
					assert.Equal(t, want, got)
				})
			}

			t.Run("missing path", func(t *testing.T) {
				_, wantErr := binary.GetPathIDInTree(treeID, "a/absent.json")
				require.ErrorIs(t, wantErr, gitinterface.ErrTreeDoesNotHavePath)

				_, gotErr := fast.GetPathIDInTree(treeID, "a/absent.json")
				assert.ErrorIs(t, gotErr, gitinterface.ErrTreeDoesNotHavePath)
			})
		})
	}
}

func TestGetReference(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)
			ids := commitChain(t, binary, "refs/heads/main", 2)

			t.Run("full reference name", func(t *testing.T) {
				want, err := binary.GetReference("refs/heads/main")
				require.Nil(t, err)
				require.Equal(t, ids[1], want)

				got, err := fast.GetReference("refs/heads/main")
				require.Nil(t, err)
				assert.Equal(t, want, got)
			})

			t.Run("missing reference reports the zero hash", func(t *testing.T) {
				want, wantErr := binary.GetReference("refs/heads/absent")
				require.ErrorIs(t, wantErr, gitstore.ErrReferenceNotFound)

				got, gotErr := fast.GetReference("refs/heads/absent")
				assert.ErrorIs(t, gotErr, gitstore.ErrReferenceNotFound)
				assert.Equal(t, want, got)
				assert.True(t, got.IsZero())
			})

			t.Run("packed refs are visible", func(t *testing.T) {
				git(t, binary, "", "pack-refs", "--all")

				got, err := fast.GetReference("refs/heads/main")
				require.Nil(t, err)
				assert.Equal(t, ids[1], got)
			})

			// rev-parse takes revision expressions go-git cannot resolve, so
			// these must fall through to the git binary.
			t.Run("revision expressions are delegated", func(t *testing.T) {
				for _, revision := range []string{"main", ids[1].String(), "refs/heads/main^{commit}", "refs/heads/main~1", "refs/heads/main@{0}"} {
					want, err := binary.GetReference(revision)
					require.Nil(t, err)

					got, err := fast.GetReference(revision)
					require.Nil(t, err, "revision %s", revision)
					assert.Equal(t, want, got)
				}
			})
		})
	}
}

func TestGetTagTarget(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)
			ids := commitChain(t, binary, "refs/heads/main", 1)

			tagID, err := binary.TagUsingSpecificKey(ids[0], "v1", "release\n", artifacts.SSHRSAPrivate)
			require.Nil(t, err)

			want, err := binary.GetTagTarget(tagID)
			require.Nil(t, err)
			require.Equal(t, ids[0], want)

			got, err := fast.GetTagTarget(tagID)
			require.Nil(t, err)
			assert.Equal(t, want, got)
		})
	}
}

func TestGetObjectSignature(t *testing.T) {
	t.Parallel()

	for _, objectFormat := range objectFormats {
		t.Run(string(objectFormat), func(t *testing.T) {
			t.Parallel()

			binary, fast := newBackends(t, objectFormat)

			treeID, err := binary.EmptyTree()
			require.Nil(t, err)

			signedID, err := binary.Commit(treeID, "refs/heads/signed", "signed\n", true)
			require.Nil(t, err)
			unsignedID, err := binary.Commit(treeID, "refs/heads/unsigned", "unsigned\n", false)
			require.Nil(t, err)
			tagID, err := binary.TagUsingSpecificKey(signedID, "v1", "release\n", artifacts.SSHRSAPrivate)
			require.Nil(t, err)

			cases := map[string]githash.Hash{
				"signed commit":   signedID,
				"unsigned commit": unsignedID,
				"signed tag":      tagID,
			}

			for name, objectID := range cases {
				t.Run(name, func(t *testing.T) {
					wantPayload, wantSignature, err := binary.GetObjectSignature(objectID)
					require.Nil(t, err)

					gotPayload, gotSignature, err := fast.GetObjectSignature(objectID)
					require.Nil(t, err)
					assert.Equal(t, wantPayload, gotPayload)
					assert.Equal(t, wantSignature, gotSignature)
				})
			}

			t.Run("signed commit actually carries a signature", func(t *testing.T) {
				_, signature, err := fast.GetObjectSignature(signedID)
				require.Nil(t, err)
				assert.NotEmpty(t, signature)
			})

			t.Run("blob is neither commit nor tag", func(t *testing.T) {
				blobID, err := binary.WriteBlob([]byte("contents"))
				require.Nil(t, err)

				_, _, wantErr := binary.GetObjectSignature(blobID)
				require.ErrorIs(t, wantErr, gitinterface.ErrNotCommitOrTag)

				_, _, gotErr := fast.GetObjectSignature(blobID)
				assert.ErrorIs(t, gotErr, gitinterface.ErrNotCommitOrTag)
			})
		})
	}
}

func TestDisabledBackendDelegates(t *testing.T) {
	t.Parallel()

	binary := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	disabled := gogitstore.New(binary, false)

	blobID, err := binary.WriteBlob([]byte("contents"))
	require.Nil(t, err)

	want, err := binary.ReadBlob(blobID)
	require.Nil(t, err)

	got, err := disabled.ReadBlob(blobID)
	require.Nil(t, err)
	assert.Equal(t, want, got)
}
