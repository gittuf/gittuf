// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestRepository(t *testing.T) {
	t.Run("repository.isBare", func(t *testing.T) {
		t.Run("bare=true", func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := CreateTestGitRepository(t, tmpDir, true)
			assert.True(t, repo.IsBare())
		})

		t.Run("bare=false", func(t *testing.T) {
			tmpDir := t.TempDir()
			repo := CreateTestGitRepository(t, tmpDir, false)
			assert.False(t, repo.IsBare())
		})
	})
}
