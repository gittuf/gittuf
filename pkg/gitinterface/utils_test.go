// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0
package gitinterface

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResetDueToError(t *testing.T) {
	tmpDir := t.TempDir()
	repo := CreateTestGitRepository(t, tmpDir, false)

	treeBuilder := NewTreeBuilder(repo)
	emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
	require.Nil(t, err)

	commitID, err := repo.Commit(emptyTreeID, "refs/heads/main", "Initial commit\n", false)
	require.Nil(t, err)

	t.Run("successful reset", func(t *testing.T) {
		cause := assert.AnError
		err := repo.ResetDueToError(cause, "refs/heads/main", commitID)
		assert.ErrorIs(t, err, cause)
	})

	t.Run("invalid ref name", func(t *testing.T) {
		cause := assert.AnError
		err := repo.ResetDueToError(cause, "invalid ref with spaces", commitID)
		assert.ErrorContains(t, err, "unable to reset")
		assert.ErrorIs(t, err, cause)
	})
}
