// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"os"
	"path/filepath"
	"testing"

	hookopts "github.com/gittuf/gittuf/experimental/gittuf/options/hooks"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUpdatePrePushHook(t *testing.T) {
	t.Run("write hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		r := &Repository{r: repo}

		err := r.UpdateHook(HookPrePush, []byte("some content"), false)
		require.NoError(t, err)

		hookFile := filepath.Join(repo.GetGitDir(), "hooks", "pre-push")
		prepushScript, err := os.ReadFile(hookFile)
		require.NoError(t, err)
		assert.Equal(t, []byte("some content"), prepushScript)
	})

	t.Run("hook exists", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		r := &Repository{r: repo}

		hookFile := filepath.Join(repo.GetGitDir(), "hooks", "pre-push")
		err := os.WriteFile(hookFile, []byte("existing hook script"), 0o700) // nolint:gosec
		require.NoError(t, err)

		err = r.UpdateHook(HookPrePush, []byte("new hook script"), false)
		var hookErr *ErrHookExists
		if assert.ErrorAs(t, err, &hookErr) {
			assert.Equal(t, HookPrePush, hookErr.HookType)
		}
	})

	t.Run("force overwrite hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		r := &Repository{r: repo}

		hookFile := filepath.Join(repo.GetGitDir(), "hooks", "pre-push")
		err := os.WriteFile(hookFile, []byte("existing hook script"), 0o700) // nolint:gosec
		require.NoError(t, err)

		err = r.UpdateHook(HookPrePush, []byte("new hook script"), true)
		assert.NoError(t, err)

		content, err := os.ReadFile(hookFile)
		assert.NoError(t, err)
		assert.Equal(t, []byte("new hook script"), content)
	})
}

func TestInvokeHooksForStage(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootKey := tufv01.NewKeyFromSSLibKey(rootSigner.MetadataKey())
	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
	targetsKey := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

	t.Run("no signer configured", func(t *testing.T) {
		// Until https://github.com/gittuf/gittuf/pull/905 is merged
		// (adding automatic signer loading from the Git configuration), this
		// test checks that there is an error if no signer is presented.
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		r := &Repository{r: repo}

		_, err := r.InvokeHooksForStage(testCtx, tuf.HookStagePreCommit, nil)
		assert.ErrorIs(t, err, sslibdsse.ErrNoSigners)
	})

	t.Run("pre-commit hook, but for different principal", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		r := &Repository{r: repo}

		hookStage := tuf.HookStagePreCommit
		hookName := "test-hook"
		environment := tuf.HookEnvironmentLua
		modules := []string{}
		principals := []string{targetsKey.KeyID}

		if err := r.InitializeRoot(testCtx, rootSigner, false); err != nil {
			t.Fatal(err)
		}
		if err := r.AddRootKey(testCtx, rootSigner, targetsKey, false); err != nil {
			t.Fatal(err)
		}
		if err := r.AddHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, hookBytes, environment, modules, principals, false); err != nil {
			t.Fatal(err)
		}

		err := r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)
		err = r.ApplyPolicy(testCtx, "", true, false)
		require.Nil(t, err)

		_, err = r.InvokeHooksForStage(testCtx, hookStage, rootSigner)
		assert.ErrorIs(t, err, ErrNoHooksFoundForPrincipal)
	})

	t.Run("pre-commit hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		r := &Repository{r: repo}

		hookStage := tuf.HookStagePreCommit
		hookName := "test-hook"
		environment := tuf.HookEnvironmentLua
		modules := []string{}
		principals := []string{rootKey.KeyID}

		if err := r.InitializeRoot(testCtx, rootSigner, false); err != nil {
			t.Fatal(err)
		}
		if err := r.AddHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, hookBytes, environment, modules, principals, false); err != nil {
			t.Fatal(err)
		}

		err := r.StagePolicy(testCtx, "", true, false)
		require.Nil(t, err)
		err = r.ApplyPolicy(testCtx, "", true, false)
		require.Nil(t, err)

		codes, err := r.InvokeHooksForStage(testCtx, hookStage, rootSigner)
		assert.Nil(t, err)
		assert.Len(t, codes, 1)
	})

	t.Run("pre-push hook", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		treeBuilder := gitinterface.NewTreeBuilder(repo)
		emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
		if err != nil {
			t.Fatal(err)
		}
		_, err = repo.Commit(emptyTreeHash, "refs/heads/main", "Initial commit\n", false)
		if err != nil {
			t.Fatal(err)
		}

		remoteTmpDir := t.TempDir()
		_ = gitinterface.CreateTestGitRepository(t, remoteTmpDir, false)

		if err := repo.CreateRemote("origin", remoteTmpDir); err != nil {
			t.Fatal(err)
		}

		r := &Repository{r: repo}

		hookStage := tuf.HookStagePrePush
		hookName := "test-hook"
		environment := tuf.HookEnvironmentLua
		modules := []string{}
		principals := []string{rootKey.KeyID}

		if err := r.InitializeRoot(testCtx, rootSigner, false); err != nil {
			t.Fatal(err)
		}
		if err := r.AddHook(testCtx, rootSigner, []tuf.HookStage{hookStage}, hookName, hookBytes, environment, modules, principals, false); err != nil {
			t.Fatal(err)
		}

		err = r.StagePolicy(testCtx, "", true, false)
		assert.Nil(t, err)
		err = r.ApplyPolicy(testCtx, "", true, false)
		assert.Nil(t, err)

		codes, err := r.InvokeHooksForStage(testCtx, hookStage, rootSigner, hookopts.WithPrePush("origin", remoteTmpDir, []string{"refs/heads/main:refs/heads/main"}))
		assert.Nil(t, err)
		assert.Len(t, codes, 1)
	})
}
