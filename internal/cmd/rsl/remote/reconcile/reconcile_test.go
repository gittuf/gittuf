// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package reconcile

import (
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestReconcile(t *testing.T) {
	t.Run("missing required argument via cobra", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "accepts 1 arg(s), received 0")
	})

	t.Run("invalid repository path", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		_, _, _, err := cmd.ExecuteCommandC(New(), "origin")
		assert.ErrorContains(t, err, "not a git repository")
	})

	t.Run("repository without remote", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		_ = gitinterface.CreateTestGitRepository(t, tmpDir, false)

		_, _, _, err := cmd.ExecuteCommandC(New(), "origin")
		assert.Error(t, err)
	})
}
