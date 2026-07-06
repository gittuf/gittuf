// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package listglobalrules

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/cmd"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestListGlobalRules(t *testing.T) {
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

	t.Run("success no rules", func(t *testing.T) {
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
		assert.Contains(t, stdout.String(), "No global rules are currently defined.")
	})

	t.Run("success with rules", func(t *testing.T) {
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

		// Add threshold global rule
		if err := repo.AddGlobalRuleThreshold(t.Context(), signer, "require-approval-for-main", []string{"git:refs/heads/main", "file:src/*"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		// Add block force pushes global rule
		if err := repo.AddGlobalRuleBlockForcePushes(t.Context(), signer, "block-force-pushes-for-main", []string{"git:refs/heads/main"}, false, trustpolicyopts.WithRSLEntry()); err != nil {
			t.Fatal(err)
		}

		_, stdout, _, err := cmd.ExecuteCommandC(New(), "--target-ref", "policy-staging")
		assert.NoError(t, err)

		expected := `Global Rule: require-approval-for-main
    Type: threshold
    Paths affected:
        file:src/*
    Refs affected:
        git:refs/heads/main
    Threshold: 1
Global Rule: block-force-pushes-for-main
    Type: block-force-pushes
    Refs affected:
        git:refs/heads/main
`

		output := strings.ReplaceAll(stdout.String(), "\r\n", "\n")
		assert.Equal(t, expected, output)
	})
}
