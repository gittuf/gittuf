// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPropagateUpstreamRepositoryContents(t *testing.T) {
	tmpDir1 := t.TempDir()
	downstreamRepository := CreateTestGitRepository(t, tmpDir1, false)

	blobAID, err := downstreamRepository.WriteBlob([]byte("a"))
	require.Nil(t, err)

	blobBID, err := downstreamRepository.WriteBlob([]byte("b"))
	require.Nil(t, err)

	downstreamTreeBuilder := NewTreeBuilder(downstreamRepository)

	// The downstream tree (if set as exists in test below) is:
	// oof/a -> blobA
	// b     -> blobB
	downstreamTreeEntries := []TreeEntry{
		NewEntryBlob("oof/a", blobAID),
		NewEntryBlob("b", blobBID),
	}
	downstreamTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries(downstreamTreeEntries)
	require.Nil(t, err)

	tmpDir2 := t.TempDir()
	upstreamRepository := CreateTestGitRepository(t, tmpDir2, true)

	_, err = upstreamRepository.WriteBlob([]byte("a"))
	require.Nil(t, err)

	_, err = upstreamRepository.WriteBlob([]byte("b"))
	require.Nil(t, err)

	upstreamTreeBuilder := NewTreeBuilder(upstreamRepository)

	// The upstream tree is:
	// a                -> blobA
	// foo/a            -> blobA
	// foo/b            -> blobB
	// foobar/foo/bar/b -> blobB

	upstreamTreeID, err := upstreamTreeBuilder.WriteTreeFromEntries([]TreeEntry{
		NewEntryBlob("a", blobAID),
		NewEntryBlob("foo/a", blobAID),
		NewEntryBlob("foo/b", blobBID),
		NewEntryBlob("foobar/foo/bar/b", blobBID),
	})
	require.Nil(t, err)

	upstreamRef := "refs/heads/main"
	upstreamCommitID, err := upstreamRepository.Commit(upstreamTreeID, upstreamRef, "Initial commit\n", false)
	require.Nil(t, err)

	tests := map[string]struct {
		localPath        string
		refExists        bool // refExists -> we must check for other files but no prior propagation has happened
		priorPropagation bool // priorPropagation -> localPath is already populated, mutually exclusive with refExists
		err              error
	}{
		"directory in root, no trailing slash, ref does not exist": {
			localPath:        "upstream",
			refExists:        false,
			priorPropagation: false,
		},
		"directory in root, trailing slash, ref does not exist": {
			localPath:        "upstream/",
			refExists:        false,
			priorPropagation: false,
		},
		"directory in root, no trailing slash, ref exists": {
			localPath:        "upstream",
			refExists:        true,
			priorPropagation: false,
		},
		"directory in root, trailing slash, ref exists": {
			localPath:        "upstream/",
			refExists:        true,
			priorPropagation: false,
		},
		"directory in root, no trailing slash, prior propagation exists": {
			localPath:        "upstream",
			refExists:        false,
			priorPropagation: true,
		},
		"directory in root, trailing slash, prior propagation exists": {
			localPath:        "upstream/",
			refExists:        false,
			priorPropagation: true,
		},
		"directory in subdirectory, no trailing slash, ref does not exist": {
			localPath:        "foo/upstream",
			refExists:        false,
			priorPropagation: false,
		},
		"directory in subdirectory, trailing slash, ref does not exist": {
			localPath:        "foo/upstream/",
			refExists:        false,
			priorPropagation: false,
		},
		"directory in subdirectory, no trailing slash, ref exists": {
			localPath:        "foo/upstream",
			refExists:        true,
			priorPropagation: false,
		},
		"directory in subdirectory, trailing slash, ref exists": {
			localPath:        "foo/upstream/",
			refExists:        true,
			priorPropagation: false,
		},
		"directory in subdirectory, no trailing slash, prior propagation exists": {
			localPath:        "foo/upstream",
			refExists:        false,
			priorPropagation: true,
		},
		"directory in subdirectory, trailing slash, prior propagation exists": {
			localPath:        "foo/upstream/",
			refExists:        false,
			priorPropagation: true,
		},
		"empty localPath": {
			err: ErrCannotPropagateIntoRootTree,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			require.False(t, test.refExists && test.priorPropagation, "refExists and priorPropagation can't both be true")

			if test.refExists {
				_, err := downstreamRepository.Commit(downstreamTreeID, testNameToRefName(name), "Initial commit\n", false)
				require.Nil(t, err)
			} else if test.priorPropagation {
				// We set the upstream path to contain the same tree as the
				// downstreamTree, so:
				// oof/a            -> blobA
				// b                -> blobB
				// <upstream>/oof/a -> blobA
				// <upstream>/b     -> blobB

				entries := []TreeEntry{NewEntryTree(test.localPath, downstreamTreeID)}
				entries = append(entries, downstreamTreeEntries...)

				rootTreeID, err := downstreamTreeBuilder.WriteTreeFromEntries(entries)
				require.Nil(t, err)

				_, err = downstreamRepository.Commit(rootTreeID, testNameToRefName(name), "Initial commit\n", false)
				require.Nil(t, err)
			}

			downstreamCommitID, err := downstreamRepository.PropagateUpstreamRepositoryContents(upstreamRepository, upstreamCommitID, testNameToRefName(name), test.localPath)
			if test.err != nil {
				assert.ErrorIs(t, err, test.err)
			} else {
				assert.Nil(t, err)

				rootTreeID, err := downstreamRepository.GetCommitTreeID(downstreamCommitID)
				require.Nil(t, err)

				itemID, err := downstreamRepository.GetPathIDInTree(test.localPath, rootTreeID)
				require.Nil(t, err)
				assert.Equal(t, upstreamTreeID, itemID)

				if test.refExists {
					// check that other items are still present
					itemID, err := downstreamRepository.GetPathIDInTree("oof/a", downstreamTreeID)
					require.Nil(t, err)
					assert.Equal(t, blobAID, itemID)

					itemID, err = downstreamRepository.GetPathIDInTree("b", downstreamTreeID)
					require.Nil(t, err)
					assert.Equal(t, blobBID, itemID)
				}

				// We don't need to similarly check when test.priorPropagation is
				// true because we already checked that those contents don't exist
				// in that localPath when we checked the tree ID patches
				// upstreamTreeID
			}
		})
	}
}
