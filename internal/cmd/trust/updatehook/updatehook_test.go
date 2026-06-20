// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package updatehook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/dev"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestUpdateHook(t *testing.T) {
	t.Run("not in dev mode", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "0")

		pOpts := &persistent.Options{}
		c := New(pOpts)

		_, _, _, err := cmd.ExecuteCommandC(c,
			"--file-path", "test.lua",
			"--hook-name", "test-hook",
			"--principal-ID", "test-principal",
			"--is-pre-commit",
		)
		assert.ErrorIs(t, err, dev.ErrNotInDevMode)
	})

	t.Run("invalid environment", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")

		pOpts := &persistent.Options{}
		c := New(pOpts)

		_, _, _, err := cmd.ExecuteCommandC(c,
			"--file-path", "test.lua",
			"--hook-name", "test-hook",
			"--principal-ID", "test-principal",
			"--is-pre-commit",
			"--env", "invalid",
		)
		assert.ErrorIs(t, err, tuf.ErrInvalidHookEnvironment)
	})

	t.Run("invalid timeout", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")

		pOpts := &persistent.Options{}
		c := New(pOpts)

		_, _, _, err := cmd.ExecuteCommandC(c,
			"--file-path", "test.lua",
			"--hook-name", "test-hook",
			"--principal-ID", "test-principal",
			"--is-pre-commit",
			"--timeout", "0",
		)
		assert.ErrorIs(t, err, gittuf.ErrInvalidHookTimeout)
	})

	t.Run("no repository", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
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

		pOpts := &persistent.Options{
			SigningKey: "dummy-key",
		}
		c := New(pOpts)

		_, _, _, err = cmd.ExecuteCommandC(c,
			"--file-path", "test.lua",
			"--hook-name", "test-hook",
			"--principal-ID", "test-principal",
			"--is-pre-commit",
		)
		assert.ErrorContains(t, err, "not a git repository")
	})

	t.Run("invalid signer", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
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

		pOpts := &persistent.Options{
			SigningKey: "non-existent-key",
		}
		c := New(pOpts)

		_, _, _, err = cmd.ExecuteCommandC(c,
			"--file-path", "test.lua",
			"--hook-name", "test-hook",
			"--principal-ID", "test-principal",
			"--is-pre-commit",
		)
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("invalid file path", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

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

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		c := New(pOpts)

		_, _, _, err = cmd.ExecuteCommandC(c,
			"--file-path", "non-existent-script.lua",
			"--hook-name", "test-hook",
			"--principal-ID", "test-principal",
			"--is-pre-commit",
		)
		assert.ErrorContains(t, err, "no such file or directory")
	})

	t.Run("success pre-commit", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		scriptPath := filepath.Join(tmpDir, "script.lua")
		if err := os.WriteFile(scriptPath, []byte("print('hello')"), 0o600); err != nil {
			t.Fatal(err)
		}

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

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		// Add the hook first so it exists to be updated
		if err := repo.AddHook(t.Context(), signer, []tuf.HookStage{tuf.HookStagePreCommit}, "test-hook", []byte("print('hello')"), tuf.HookEnvironmentLua, []string{"test-principal"}, 10, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		c := New(pOpts)

		_, _, _, err = cmd.ExecuteCommandC(c,
			"--file-path", scriptPath,
			"--hook-name", "test-hook",
			"--principal-ID", "new-principal",
			"--is-pre-commit",
		)
		assert.NoError(t, err)
	})

	t.Run("success pre-push", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		scriptPath := filepath.Join(tmpDir, "script.lua")
		if err := os.WriteFile(scriptPath, []byte("print('hello')"), 0o600); err != nil {
			t.Fatal(err)
		}

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

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		// Add the hook first so it exists to be updated
		if err := repo.AddHook(t.Context(), signer, []tuf.HookStage{tuf.HookStagePrePush}, "test-hook", []byte("print('hello')"), tuf.HookEnvironmentLua, []string{"test-principal"}, 10, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		c := New(pOpts)

		_, _, _, err = cmd.ExecuteCommandC(c,
			"--file-path", scriptPath,
			"--hook-name", "test-hook",
			"--principal-ID", "new-principal",
			"--is-pre-push",
		)
		assert.NoError(t, err)
	})

	t.Run("success with RSL entry", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		scriptPath := filepath.Join(tmpDir, "script.lua")
		if err := os.WriteFile(scriptPath, []byte("print('hello')"), 0o600); err != nil {
			t.Fatal(err)
		}

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

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}
		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		// Add the hook first so it exists to be updated
		if err := repo.AddHook(t.Context(), signer, []tuf.HookStage{tuf.HookStagePreCommit}, "test-hook", []byte("print('hello')"), tuf.HookEnvironmentLua, []string{"test-principal"}, 10, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}
		c := New(pOpts)

		_, _, _, err = cmd.ExecuteCommandC(c,
			"--file-path", scriptPath,
			"--hook-name", "test-hook",
			"--principal-ID", "new-principal",
			"--is-pre-commit",
		)
		assert.NoError(t, err)
	})
}
