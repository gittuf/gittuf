// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package record

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecord(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		_, _, _, err = cmd.ExecuteCommandC(New(), "main", "--local-only")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("missing arguments", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		_, _, _, err = cmd.ExecuteCommandC(New(), "--local-only")
		assert.ErrorContains(t, err, "accepts 1 arg(s), received 0")
	})

	t.Run("missing remote-name or local-only", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		_, _, _, err = cmd.ExecuteCommandC(New(), "main")
		assert.ErrorContains(t, err, "at least one of the flags in the group [remote-name local-only] is required")
	})

	t.Run("both remote-name and local-only", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		_, _, _, err = cmd.ExecuteCommandC(New(), "main", "--local-only", "--remote-name", "origin")
		assert.ErrorContains(t, err, "if any flags in the group [remote-name local-only] are set")
	})

	t.Run("no signing key configured", func(t *testing.T) {
		tmpDir := t.TempDir()
		r := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		cwd, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		defer func() {
			_ = os.Chdir(cwd)
		}()

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		err = r.SetGitConfig("user.signingkey", "")
		require.NoError(t, err)

		treeBuilder := gitinterface.NewTreeBuilder(r)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)

		_, err = r.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		require.NoError(t, err)

		_, _, _, err = cmd.ExecuteCommandC(New(), "main", "--local-only")
		assert.ErrorIs(t, err, gitinterface.ErrSigningKeyNotSpecified)
	})
}
