// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package apply

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/cmd"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestApply(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		_, _, _, err = cmd.ExecuteCommandC(New(), "--local-only")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repo, err := gittuf.LoadRepository(tmpDir)
		require.NoError(t, err)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		fromRef := "refs/heads/main"
		targetTagRef := "refs/tags/v1"

		treeBuilder := gitinterface.NewTreeBuilder(repo.GetGitRepository())
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		_, err = repo.GetGitRepository().Commit(emptyTreeID, fromRef, "Initial commit\n", false)
		require.NoError(t, err)
		require.NoError(t, repo.RecordRSLEntryForReference(t.Context(), fromRef, false, rslopts.WithRecordLocalOnly()))

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)

		require.NoError(t, repo.AddReferenceAuthorization(t.Context(), signer, targetTagRef, fromRef, false))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		_, _, _, err = cmd.ExecuteCommandC(New(), "--local-only")
		assert.NoError(t, err)
	})

	t.Run("remote not defined in repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repo, err := gittuf.LoadRepository(tmpDir)
		require.NoError(t, err)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		fromRef := "refs/heads/main"
		targetTagRef := "refs/tags/v1"

		treeBuilder := gitinterface.NewTreeBuilder(repo.GetGitRepository())
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		require.NoError(t, err)
		_, err = repo.GetGitRepository().Commit(emptyTreeID, fromRef, "Initial commit\n", false)
		require.NoError(t, err)
		require.NoError(t, repo.RecordRSLEntryForReference(t.Context(), fromRef, false, rslopts.WithRecordLocalOnly()))

		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)

		require.NoError(t, repo.AddReferenceAuthorization(t.Context(), signer, targetTagRef, fromRef, false))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck

		require.NoError(t, os.Chdir(tmpDir))

		_, _, _, err = cmd.ExecuteCommandC(New(), "origin")
		assert.Error(t, err)
	})
}
