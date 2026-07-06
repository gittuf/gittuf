// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listrules

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
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestListRules(t *testing.T) {
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

		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.AddPrincipalToTargets(t.Context(), signer, policy.TargetsRoleName, []tuf.Principal{newKey}, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "protect-main", []string{newKey.ID()}, []string{"git:refs/heads/main", "file:path/to/file"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.AddDelegation(t.Context(), signer, policy.TargetsRoleName, "protect-tags", []string{newKey.ID()}, []string{"git:refs/tags/"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		if err := repo.ApplyPolicy(t.Context(), "", true, false); err != nil {
			t.Fatal(err)
		}

		_, stdout, _, err := cmd.ExecuteCommandC(New())
		assert.NoError(t, err)

		out := strings.ReplaceAll(stdout.String(), "\r\n", "\n")
		expectedProtectMain := fmt.Sprintf(
			"Rule protect-main:\n    Paths affected:\n        file:path/to/file\n    Refs affected:\n        git:refs/heads/main\n    Authorized keys:\n        %s\n    Required valid signatures: 1",
			newKey.ID(),
		)
		expectedProtectTags := fmt.Sprintf(
			"Rule protect-tags:\n    Refs affected:\n        git:refs/tags/\n    Authorized keys:\n        %s\n    Required valid signatures: 1",
			newKey.ID(),
		)
		assert.Contains(t, out, expectedProtectMain)
		assert.Contains(t, out, expectedProtectTags)
	})

	t.Run("invalid target ref", func(t *testing.T) {
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

		command := New()
		command.SetArgs([]string{"--target-ref", "refs/gittuf/invalid"})
		_, _, _, err = cmd.ExecuteCommandC(command)
		assert.ErrorContains(t, err, "unable to find RSL entry")
	})
}
