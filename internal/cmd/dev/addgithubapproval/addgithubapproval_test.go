// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addgithubapproval

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestAddGitHubApproval(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "-k", "test-key", "--repository", "owner/repo", "--pull-request-number", "1", "--review-ID", "123", "--approver", "alice")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("missing required flag signing-key", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New(), "--repository", "owner/repo", "--pull-request-number", "1", "--review-ID", "123", "--approver", "alice")
		assert.ErrorContains(t, err, `required flag(s) "signing-key" not set`)
	})

	t.Run("missing required flag repository", func(t *testing.T) {
		_, _, _, err := cmd.ExecuteCommandC(New(), "-k", "test-key", "--pull-request-number", "1", "--review-ID", "123", "--approver", "alice")
		assert.ErrorContains(t, err, `required flag(s) "repository" not set`)
	})

	t.Run("invalid repository format", func(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "-k", "test-key", "--repository", "invalid-format", "--pull-request-number", "1", "--review-ID", "123", "--approver", "alice")
		assert.ErrorContains(t, err, "invalid format for repository, must be {owner}/{repo}")
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "-k", "non-existent-key", "--repository", "owner/repo", "--pull-request-number", "1", "--review-ID", "123", "--approver", "alice")
		assert.ErrorContains(t, err, "ssh-keygen")
	})
}
