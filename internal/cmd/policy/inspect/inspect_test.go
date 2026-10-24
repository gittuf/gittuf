// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package inspect

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInspect(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()

		t.Chdir(tmpDir)

		_, _, _, err := cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		t.Chdir(tmpDir)

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}

		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}

		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		newKey, err := gittuf.LoadPublicKey(keyPath + ".pub")
		if err != nil {
			t.Fatal(err)
		}

		require.NoError(t, repo.AddTopLevelTargetsKey(t.Context(), signer, newKey, false))

		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false))

		_, stdout, _, err := cmd.ExecuteCommandC(New())
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), `"type": "targets"`)

		commitID, err := repo.GetGitRepository().GetReference(policy.PolicyStagingRef)
		require.NoError(t, err)

		// Test full commit SHA
		_, stdout, _, err = cmd.ExecuteCommandC(New(), "--revision", commitID.String())
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), `"type": "targets"`)

		// Test short commit SHA
		_, stdout, _, err = cmd.ExecuteCommandC(New(), "--revision", commitID.String()[:7])
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), `"type": "targets"`)

		// Test full ref name
		_, stdout, _, err = cmd.ExecuteCommandC(New(), "--revision", policy.PolicyStagingRef)
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), `"type": "targets"`)

		// Test short gittuf ref name
		_, stdout, _, err = cmd.ExecuteCommandC(New(), "--revision", "policy-staging")
		assert.NoError(t, err)
		assert.Contains(t, stdout.String(), `"type": "targets"`)

		// Test non-existent revision
		_, _, _, err = cmd.ExecuteCommandC(New(), "--revision", "non-existent-revision")
		assert.ErrorContains(t, err, "unable to resolve revision 'non-existent-revision'")
	})

	t.Run("invalid policy name", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		t.Chdir(tmpDir)

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}

		signer, err := gittuf.LoadSigner(repo, keyPath)
		if err != nil {
			t.Fatal(err)
		}

		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

		command := New()
		command.SetArgs([]string{"--policy-name", "does-not-exist"})
		_, _, _, err = cmd.ExecuteCommandC(command)
		assert.ErrorContains(t, err, "unable to find requested metadata file")
	})
}
