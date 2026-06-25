// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listhooks

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/dev"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestListHooks(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

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

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("uninitialized policy", func(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to find RSL entry")
	})

	t.Run("success empty", func(t *testing.T) {
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

		_, stdout, _, err := cmd.ExecuteCommandC(New(), "--target-ref", "policy-staging")
		assert.NoError(t, err)
		assert.NotContains(t, stdout.String(), "Hook")
	})

	t.Run("success with hooks", func(t *testing.T) {
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

		keyID, err := signer.KeyID()
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.AddHook(t.Context(), signer, []tuf.HookStage{tuf.HookStagePreCommit}, "test-hook", []byte("echo test"), tuf.HookEnvironmentLua, []string{keyID}, 100, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		_, stdout, _, err := cmd.ExecuteCommandC(New(), "--target-ref", "policy-staging")
		assert.NoError(t, err)

		hookStages, err := repo.ListHooks(t.Context(), "policy-staging")
		assert.NoError(t, err)
		assert.Len(t, hookStages[tuf.HookStagePreCommit], 1)
		hook := hookStages[tuf.HookStagePreCommit][0]

		expectedPrefix := fmt.Sprintf(`Stage preCommit:
    Hook 'test-hook':
        Principal IDs:
            %s
        Hashes:`, keyID)

		expectedSuffix := `        Environment:
            lua
        Timeout:
            100`

		output := strings.ReplaceAll(stdout.String(), "\r\n", "\n")
		assert.Contains(t, output, expectedPrefix)
		assert.Contains(t, output, expectedSuffix)
		for algo, hash := range hook.GetHashes() {
			assert.Contains(t, output, fmt.Sprintf("            %s: %s", algo, hash))
		}
	})
}
