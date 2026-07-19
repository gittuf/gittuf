// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhook

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/dev"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestAddHook(t *testing.T) {
	t.Run("not in dev mode", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "0")

		pOpts := &persistent.Options{}
		_, _, _, err := cmd.ExecuteCommandC(New(pOpts),
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
		_, _, _, err := cmd.ExecuteCommandC(New(pOpts),
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
		_, _, _, err := cmd.ExecuteCommandC(New(pOpts),
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
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		pOpts := &persistent.Options{
			SigningKey: "dummy-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
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
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		pOpts := &persistent.Options{
			SigningKey: "non-existent-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
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
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--file-path", "non-existent-script.lua",
			"--hook-name", "test-hook",
			"--principal-ID", "test-principal",
			"--is-pre-commit",
		)
		assert.ErrorIs(t, err, os.ErrNotExist)
	})

	t.Run("success pre-commit", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		scriptPath := filepath.Join(tmpDir, "script.lua")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('hello')"), 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--file-path", scriptPath,
			"--hook-name", "test-hook",
			"--principal-ID", "test-principal",
			"--is-pre-commit",
		)
		assert.NoError(t, err)
	})

	t.Run("success pre-push", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		scriptPath := filepath.Join(tmpDir, "script.lua")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('hello')"), 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--file-path", scriptPath,
			"--hook-name", "test-hook",
			"--principal-ID", "test-principal",
			"--is-pre-push",
		)
		assert.NoError(t, err)
	})

	t.Run("success with RSL entry", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		scriptPath := filepath.Join(tmpDir, "script.lua")
		require.NoError(t, os.WriteFile(scriptPath, []byte("print('hello')"), 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--file-path", scriptPath,
			"--hook-name", "test-hook",
			"--principal-ID", "test-principal",
			"--is-pre-commit",
		)
		assert.NoError(t, err)
	})
}
