// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifyref

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestVerifyRef(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "refs/heads/main")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("mutually exclusive flags", func(t *testing.T) {
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

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		_, _, _, err = cmd.ExecuteCommandC(New(), "refs/heads/main", "--latest-only", "--from-entry", "some-entry-id")
		assert.ErrorContains(t, err, "if any flags in the group [latest-only from-entry] are set none of the others can be")
	})

	t.Run("from entry not in dev mode", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "0")

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

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		_, _, _, err = cmd.ExecuteCommandC(New(), "refs/heads/main", "--from-entry", "some-entry-id")
		assert.ErrorIs(t, err, dev.ErrNotInDevMode)
	})

	t.Run("from entry in dev mode", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")

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

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		_, _, _, err = cmd.ExecuteCommandC(New(), "refs/heads/main", "--from-entry", "0000000000000000000000000000000000000000")
		assert.Error(t, err)
		assert.NotEqual(t, dev.ErrNotInDevMode, err)
	})

	t.Run("uninitialized repository", func(t *testing.T) {
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

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		_, _, _, err = cmd.ExecuteCommandC(New(), "refs/heads/main", "--latest-only")
		assert.Error(t, err)
	})
}
