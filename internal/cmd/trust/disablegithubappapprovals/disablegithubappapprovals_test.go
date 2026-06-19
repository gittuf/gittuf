// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package disablegithubappapprovals

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestDisableGitHubAppApprovals(t *testing.T) {
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

		pOpts := &persistent.Options{
			SigningKey: "dummy-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts))
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("invalid signer", func(t *testing.T) {
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

		pOpts := &persistent.Options{
			SigningKey: "non-existent-key",
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts))
		assert.ErrorContains(t, err, "failed to run command")
	})

	t.Run("success already untrusted", func(t *testing.T) {
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

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
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

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", "github-app")
		assert.NoError(t, err)
	})

	t.Run("success disable trusted app", func(t *testing.T) {
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

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
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

		key, err := gittuf.LoadPublicKey(keyPath)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.AddGitHubApp(t.Context(), signer, "github-app", key, false)
		assert.NoError(t, err)

		err = repo.TrustGitHubApp(t.Context(), signer, "github-app", false)
		assert.NoError(t, err)

		err = repo.StagePolicy(t.Context(), "", true, false)
		assert.NoError(t, err)

		state, err := policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		assert.NoError(t, err)
		rootMetadata, err := state.GetRootMetadata(false)
		assert.NoError(t, err)
		assert.True(t, rootMetadata.IsGitHubAppApprovalTrusted("github-app"))

		pOpts := &persistent.Options{
			SigningKey: keyPath,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", "github-app")
		assert.NoError(t, err)

		err = repo.StagePolicy(t.Context(), "", true, false)
		assert.NoError(t, err)

		state, err = policy.LoadCurrentState(t.Context(), repo.GetGitRepository(), policy.PolicyStagingRef)
		assert.NoError(t, err)
		rootMetadata, err = state.GetRootMetadata(false)
		assert.NoError(t, err)
		assert.False(t, rootMetadata.IsGitHubAppApprovalTrusted("github-app"))
	})

	t.Run("success disable trusted app with RSL entry", func(t *testing.T) {
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

		keyPath := filepath.Join(tmpDir, "test-key")
		if err := os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600); err != nil {
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

		key, err := gittuf.LoadPublicKey(keyPath)
		if err != nil {
			t.Fatal(err)
		}

		err = repo.AddGitHubApp(t.Context(), signer, "github-app", key, false)
		assert.NoError(t, err)

		err = repo.TrustGitHubApp(t.Context(), signer, "github-app", false)
		assert.NoError(t, err)

		err = repo.StagePolicy(t.Context(), "", true, false)
		assert.NoError(t, err)

		pOpts := &persistent.Options{
			SigningKey:   keyPath,
			WithRSLEntry: true,
		}
		_, _, _, err = cmd.ExecuteCommandC(New(pOpts), "--app-name", "github-app")
		assert.NoError(t, err)
	})
}
