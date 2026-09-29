// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifynetwork

import (
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestVerifyNetwork(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("uninitialized repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.Error(t, err)
	})
}
