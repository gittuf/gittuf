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
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestCacheDelete(t *testing.T) {
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

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.ErrorContains(t, err, "unable to identify git directory")
	})

	t.Run("success", func(t *testing.T) {
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

		repo, err := gittuf.LoadRepository(".")
		if err != nil {
			t.Fatal(err)
		}

		gitRepo := repo.GetGitRepository()
		err = rsl.NewReferenceEntry(policy.PolicyRef, gitinterface.ZeroHash).Commit(gitRepo, false)
		if err != nil {
			t.Fatal(err)
		}

		// Populate cache first so delete does not fail with cache-not-found
		err = repo.PopulateCache()
		if err != nil {
			t.Fatal(err)
		}

		_, _, _, err = cmd.ExecuteCommandC(New())
		assert.NoError(t, err)

		_, err = gitRepo.GetReference(cache.Ref)
		assert.ErrorIs(t, err, gitinterface.ErrReferenceNotFound)
	})
}
