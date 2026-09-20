// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package log

import (
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestLog(t *testing.T) {
	t.Run("invalid repository path", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "not a git repository")
	})

	t.Run("repository without RSL initialized", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		_ = gitinterface.CreateTestGitRepository(t, tmpDir, false)

		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to find RSL entry")
	})
}
