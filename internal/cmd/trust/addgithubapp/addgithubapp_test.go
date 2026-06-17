// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addgithubapp

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestAddGitHubApp(t *testing.T) {
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
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--app-name", "test-app",
			"--app-key", "dummy-key",
		)
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
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--app-name", "test-app",
			"--app-key", "dummy-key",
		)
		assert.Error(t, err)
	})

	t.Run("invalid app key", func(t *testing.T) {
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

		// Initialize the repository first
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

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--app-name", "test-app",
			"--app-key", "non-existent-key",
		)
		assert.Error(t, err)
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		appKeyPath := filepath.Join(tmpDir, "app-key")
		if err := os.WriteFile(appKeyPath, artifacts.SSHRSAPrivate, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(appKeyPath+".pub", artifacts.SSHRSAPublicSSH, 0o600); err != nil {
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

		// Initialize the repository first
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

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--app-name", "test-app",
			"--app-key", appKeyPath+".pub",
		)
		assert.NoError(t, err)
	})

	t.Run("success with RSL entry", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
			t.Fatal(err)
		}

		appKeyPath := filepath.Join(tmpDir, "app-key")
		if err := os.WriteFile(appKeyPath, artifacts.SSHRSAPrivate, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(appKeyPath+".pub", artifacts.SSHRSAPublicSSH, 0o600); err != nil {
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

		// Initialize the repository first
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

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}

		_, _, _, err = cmd.ExecuteCommandC(New(pOpts),
			"--app-name", "test-app",
			"--app-key", appKeyPath+".pub",
		)
		assert.NoError(t, err)
	})
}
