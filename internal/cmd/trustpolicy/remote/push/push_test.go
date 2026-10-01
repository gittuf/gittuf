// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package push

import (
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestPush(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		t.Chdir(tmpDir)

		_, _, _, err := cmd.ExecuteCommandC(New(), "origin")
		assert.ErrorContains(t, err, "not a git repository")
	})

	t.Run("invalid remote", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		t.Chdir(tmpDir)

		_, _, _, err := cmd.ExecuteCommandC(New(), "non-existent-remote")
		assert.ErrorContains(t, err, "unable to push policy")
	})
}
