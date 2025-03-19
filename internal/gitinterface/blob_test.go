// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepositoryReadBlob(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	contents := []byte("test file read")
	expectedBlobID, err := NewHash("2ecdd330475d93568ed27f717a84a7fe207d1c58")
	require.Nil(t, err)

	blobID, err := repo.WriteBlob(contents)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedBlobID, blobID)

	t.Run("read existing blob", func(t *testing.T) {
		readContents, err := repo.ReadBlob(blobID)
		assert.Nil(t, err)
		assert.Equal(t, contents, readContents)
	})

	t.Run("read non-existing blob", func(t *testing.T) {
		_, err := repo.ReadBlob(ZeroHash)
		assert.NotNil(t, err)
	})
}

func TestRepositoryWriteBlob(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	contents := []byte("test file write")
	expectedBlobID, err := NewHash("999c05e9578e5d244920306842f516789a2498f7")
	require.Nil(t, err)

	blobID, err := repo.WriteBlob(contents)
	assert.Nil(t, err)
	assert.Equal(t, expectedBlobID, blobID)
}

func TestIsBlobBinary(t *testing.T) {
	tempDir := t.TempDir()
	repo := CreateTestGitRepository(t, tempDir, false)

	t.Run("text blob", func(t *testing.T) {
		contents := []byte("test text file write")
		expectedBlobID, err := NewHash("df04a970587839ca085545362089fda43fb0e53b")
		require.Nil(t, err)

		blobID, err := repo.WriteBlob(contents)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectedBlobID, blobID)

		treeBuilder := NewTreeBuilder(repo)
		_, err = treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		isBinary, err := repo.IsBlobBinary(blobID)
		assert.Nil(t, err)
		assert.False(t, isBinary)
	})

	t.Run("binary blob", func(t *testing.T) {
		contents := artifacts.GittufLogo
		expectedBlobID, err := NewHash("1bd6e2e70e2fff7be72ac7160b44049992d15507")
		require.Nil(t, err)

		blobID, err := repo.WriteBlob(contents)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, expectedBlobID, blobID)

		treeBuilder := NewTreeBuilder(repo)
		_, err = treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}

		isBinary, err := repo.IsBlobBinary(blobID)
		assert.Nil(t, err)
		assert.True(t, isBinary)
	})
}
