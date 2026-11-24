// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTrustLifecycleActions(t *testing.T) {
	tmpDir := t.TempDir()
	t.Chdir(tmpDir)

	gitinterface.CreateTestGitRepository(t, tmpDir, false)

	keyPath := filepath.Join(tmpDir, "test-key")
	require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
	require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

	repo, err := gittuf.LoadRepository(".")
	require.NoError(t, err)
	signer, err := gittuf.LoadSigner(repo, keyPath)
	require.NoError(t, err)
	require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))

	m := &model{
		ctx: t.Context(),
		options: &options{
			p: &persistent.Options{SigningKey: keyPath},
		},
		screen: screenTrustLifecycle,
	}
	s := &trustLifecycleScreen{}

	t.Run("stage trust changes succeeds locally", func(t *testing.T) {
		s.selectedAction = trustLifecycleActionStage
		runSelectedLifecycleAction(s, m)

		assert.Nil(t, m.errorDialog)
		assert.Equal(t, "Trust changes staged!", m.footer)
	})

	t.Run("apply trust changes succeeds locally", func(t *testing.T) {
		s.selectedAction = trustLifecycleActionApply
		runSelectedLifecycleAction(s, m)

		assert.Nil(t, m.errorDialog)
		assert.Equal(t, "Trust changes applied!", m.footer)
	})
}
