// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package authorize

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/attest/persistent"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestAuthorize(t *testing.T) {
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

		pOpts := &persistent.Options{
			SigningKey: "dummy-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--from-ref", "refs/heads/main", "refs/tags/v1")
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("invalid signer", func(t *testing.T) {
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
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--from-ref", "refs/heads/main", "refs/tags/v1")
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("insufficient parameters for revoking", func(t *testing.T) {
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

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--from-ref", "refs/heads/main", "--revoke", "refs/tags/v1")
		assert.ErrorContains(t, err, "insufficient parameters for revoking authorization, requires <targetRef> <fromID> <targetTreeID>")
	})

	t.Run("success authorize", func(t *testing.T) {
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
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--from-ref", fromRef, targetTagRef)
		assert.NoError(t, err)
	})

	t.Run("success authorize with RSL entry", func(t *testing.T) {
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
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--from-ref", fromRef, targetTagRef)
		assert.NoError(t, err)
	})

	t.Run("success revoke", func(t *testing.T) {
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
		initialCommitID, err := repo.GetGitRepository().Commit(emptyTreeID, fromRef, "Initial commit\n", false)
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

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--from-ref", fromRef, "--revoke", targetTagRef, gitinterface.ZeroHash.String(), initialCommitID.String())
		assert.NoError(t, err)
	})
}
