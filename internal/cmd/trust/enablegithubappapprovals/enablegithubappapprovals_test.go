// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package enablegithubappapprovals

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestEnableGitHubAppApprovals(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts))
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

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts))
		assert.Error(t, err)
	})

	t.Run("uninitialized policy", func(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", tuf.GitHubAppRoleName)
		assert.ErrorContains(t, err, "reference not found")
	})

	t.Run("uninitialized policy with RSL", func(t *testing.T) {
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
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", tuf.GitHubAppRoleName)
		assert.ErrorContains(t, err, "reference not found")
	})

	t.Run("success already trusted", func(t *testing.T) {
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

		if err := repo.InitializeRoot(t.Context(), signer, false); err != nil {
			t.Fatal(err)
		}

		appKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}

		// Add GitHub App and set trusted to true
		if err := repo.AddGitHubApp(t.Context(), signer, tuf.GitHubAppRoleName, appKey, false); err != nil {
			t.Fatal(err)
		}
		if err := repo.TrustGitHubApp(t.Context(), signer, tuf.GitHubAppRoleName, false); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", tuf.GitHubAppRoleName)
		assert.NoError(t, err)
	})

	t.Run("success enable untrusted app", func(t *testing.T) {
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

		if err := repo.InitializeRoot(t.Context(), signer, false); err != nil {
			t.Fatal(err)
		}

		appKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}

		// Add GitHub App (untrusted by default)
		if err := repo.AddGitHubApp(t.Context(), signer, tuf.GitHubAppRoleName, appKey, false); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", tuf.GitHubAppRoleName)
		assert.NoError(t, err)
	})

	t.Run("success enable untrusted app with RSL entry", func(t *testing.T) {
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

		if err := repo.InitializeRoot(t.Context(), signer, false); err != nil {
			t.Fatal(err)
		}

		appKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}

		// Add GitHub App (untrusted by default)
		if err := repo.AddGitHubApp(t.Context(), signer, tuf.GitHubAppRoleName, appKey, false); err != nil {
			t.Fatal(err)
		}

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", tuf.GitHubAppRoleName)
		assert.NoError(t, err)
	})
}
