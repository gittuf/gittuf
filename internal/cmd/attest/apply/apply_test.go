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
)

func TestApply(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "--local-only")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repo, err := gittuf.LoadRepository(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		fromRef := "refs/heads/main"
		targetTagRef := "refs/tags/v1"

		treeBuilder := gitinterface.NewTreeBuilder(repo.GetGitRepository())
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = repo.GetGitRepository().Commit(emptyTreeID, fromRef, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.RecordRSLEntryForReference(t.Context(), fromRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.AddReferenceAuthorization(t.Context(), signer, targetTagRef, fromRef, false); err != nil {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "--local-only")
		assert.NoError(t, err)
	})

	t.Run("remote not defined in repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)
		repo, err := gittuf.LoadRepository(tmpDir)
		if err != nil {
			t.Fatal(err)
		}

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		fromRef := "refs/heads/main"
		targetTagRef := "refs/tags/v1"

		treeBuilder := gitinterface.NewTreeBuilder(repo.GetGitRepository())
		emptyTreeID, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = repo.GetGitRepository().Commit(emptyTreeID, fromRef, "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}
		if err := repo.RecordRSLEntryForReference(t.Context(), fromRef, false, rslopts.WithRecordLocalOnly()); err != nil {
			t.Fatal(err)
		}

		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.AddReferenceAuthorization(t.Context(), signer, targetTagRef, fromRef, false); err != nil {
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

		_, _, _, err = cmd.ExecuteCommandC(New(), "origin")
		assert.Error(t, err)
	})
}
