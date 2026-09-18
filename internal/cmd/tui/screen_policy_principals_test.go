// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyPrincipalsFormSubmit(t *testing.T) {
	t.Run("custom metadata value containing '='", func(t *testing.T) {
		tmpDir := t.TempDir()
		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		cwd, err := os.Getwd()
		require.NoError(t, err)
		defer os.Chdir(cwd) //nolint:errcheck
		require.NoError(t, os.Chdir(tmpDir))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))
		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false))

		m := &model{
			ctx: t.Context(),
			options: &options{
				p:          &persistent.Options{SigningKey: keyPath},
				policyName: policy.TargetsRoleName,
			},
			policyPrincipalsScreen: policyPrincipalsScreen{
				list: newMenuList("Policy Principals", []list.Item{}, newDelegate(4)),
			},
		}

		f := &m.policyPrincipalsFormScreen
		f.initInputs("Add Person")
		f.inputs[0].SetValue("jane.doe@example.com")
		f.inputs[1].SetValue(keyPath + ".pub")
		f.inputs[3].SetValue("profile=https://example.com/u?ref=1")
		f.focusIndex = len(f.inputs) - 1

		f.handleFormSubmit(m)

		assert.Empty(t, m.errorMsg)
		assert.Nil(t, m.errorDialog)
		assert.Equal(t, "Principal added successfully!", m.footer)
	})
}
