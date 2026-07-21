// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package delete

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cache"
	"github.com/gittuf/gittuf/internal/cmd"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestCacheDelete(t *testing.T) {
	t.Run("no repository", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(currentDir) //nolint:errcheck

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("success", func(t *testing.T) {
		tmpDir := t.TempDir()
		currentDir, err := os.Getwd()
		require.NoError(t, err)
		require.NoError(t, os.Chdir(tmpDir))
		defer os.Chdir(currentDir) //nolint:errcheck

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)

		gitRepo := repo.GetGitRepository()
		err = rsl.NewReferenceEntry(policy.PolicyRef, gitinterface.ZeroHash).Commit(gitRepo, false)
		require.NoError(t, err)

		// Populate cache first so delete does not fail with cache-not-found
		err = repo.PopulateCache()
		require.NoError(t, err)

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.NoError(t, err)

		_, err = gitRepo.GetReference(cache.Ref)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)
	})
}
