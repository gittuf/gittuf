// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package updatepropagationdirective

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
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePropagationDirective(t *testing.T) {
	t.Run("not in dev mode", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "0")

		pOpts := &persistent.Options{}
		c := New(pOpts)

		_, _, _, err := cmd.ExecuteCommandC(c,
			"--name", "test-directive",
			"--from-repository", "origin",
			"--from-reference", "refs/heads/main",
			"--into-reference", "refs/heads/main",
			"--into-path", "bar",
		)
		assert.ErrorIs(t, err, dev.ErrNotInDevMode)
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
			"--name", "test-directive",
			"--from-repository", "origin",
			"--from-reference", "refs/heads/main",
			"--into-reference", "refs/heads/main",
			"--into-path", "bar",
		)
		assert.ErrorContains(t, err, "unable to identify git directory")
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
			"--name", "test-directive",
			"--from-repository", "origin",
			"--from-reference", "refs/heads/main",
			"--into-reference", "refs/heads/main",
			"--into-path", "bar",
		)
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("success", func(t *testing.T) {
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

		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()))

		require.NoError(t, repo.AddPropagationDirective(t.Context(), signer, "test-directive", "origin", "refs/heads/main", "foo", "refs/heads/main", "bar", false, trustpolicyopts.WithRSLEntry()))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--name", "test-directive",
			"--from-repository", "origin-new",
			"--from-reference", "refs/heads/dev",
			"--from-path", "new-foo",
			"--into-reference", "refs/heads/dev",
			"--into-path", "new-bar",
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

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)

		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()))

		require.NoError(t, repo.AddPropagationDirective(t.Context(), signer, "test-directive", "origin", "refs/heads/main", "foo", "refs/heads/main", "bar", false, trustpolicyopts.WithRSLEntry()))

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--name", "test-directive",
			"--from-repository", "origin-new",
			"--from-reference", "refs/heads/dev",
			"--from-path", "new-foo",
			"--into-reference", "refs/heads/dev",
			"--into-path", "new-bar",
		)
		assert.NoError(t, err)
	})
}
