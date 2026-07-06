// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package verifymergeable

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestVerifyMergeable(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "--base-branch", "main", "--feature-branch", "feature")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("missing required flags", func(t *testing.T) {
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

		// Test missing feature-branch
		_, _, _, err = cmd.ExecuteCommandC(New(), "--base-branch", "main")
		assert.ErrorContains(t, err, "required flag(s)")
		assert.ErrorContains(t, err, "feature-branch")

		// Test missing base-branch
		_, _, _, err = cmd.ExecuteCommandC(New(), "--feature-branch", "feature")
		assert.ErrorContains(t, err, "required flag(s)")
		assert.ErrorContains(t, err, "base-branch")
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "--base-branch", "main", "--feature-branch", "feature")
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)
	})

	t.Run("bypass RSL", func(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "--base-branch", "main", "--feature-branch", "feature", "--bypass-RSL")
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)
	})
}
