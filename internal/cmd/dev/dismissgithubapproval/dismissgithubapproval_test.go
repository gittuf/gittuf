// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dismissgithubapproval

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestDismissGitHubApproval(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(currentDir)
		}()

		_, _, _, err = cmd.ExecuteCommandC(New(), "-k", "test-key", "--dismiss-approver", "alice", "--review-ID", "123")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("missing required flag signing-key", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New(), "--dismiss-approver", "alice", "--review-ID", "123")
		assert.ErrorContains(t, err, `required flag(s) "signing-key" not set`)
	})

	t.Run("missing required flag dismiss-approver", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New(), "-k", "test-key", "--review-ID", "123")
		assert.ErrorContains(t, err, `required flag(s) "dismiss-approver" not set`)
	})

	t.Run("loading signer error", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(currentDir)
		}()

		_, _, _, err = cmd.ExecuteCommandC(New(), "-k", "non-existent-key", "--dismiss-approver", "alice", "--review-ID", "123")
		assert.ErrorContains(t, err, "ssh-keygen")
	})
}
