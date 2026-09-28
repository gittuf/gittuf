// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/experimental/gittuf"
	rootopts "github.com/gittuf/gittuf/experimental/gittuf/options/root"
	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/require"
)

func TestGetGlobalRulesForRef(t *testing.T) {
	tmpDir := t.TempDir()
	gitRepo := gitinterface.CreateTestGitRepository(t, tmpDir, false)
	t.Setenv(dev.DevModeKey, "1")
	keyPath := filepath.Join(tmpDir, "test-key")
	require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
	require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))
	signer, err := ssh.NewSignerFromFile(keyPath)
	require.NoError(t, err)
	repo, err := gittuf.LoadRepository(tmpDir)
	require.NoError(t, err)
	require.NoError(t, repo.InitializeRoot(t.Context(), signer, false, rootopts.WithRSLEntry()))

	patterns := []string{"git:refs/heads/main"}
	require.NoError(t, repo.AddGlobalRuleThreshold(t.Context(), signer, "local-rule", patterns, 2, false, trustpolicyopts.WithRSLEntry()))
	controllerLocation := "https://example.com/controller"
	require.NoError(t, repo.AddControllerRepository(t.Context(), signer, "controller", controllerLocation, nil, false, trustpolicyopts.WithRSLEntry()))
	controllerRoot := tufv01.NewRootMetadata()
	require.NoError(t, controllerRoot.AddGlobalRule(tufv01.NewGlobalRuleThreshold("controller-rule", patterns, 3)))
	controllerEnvelope, err := dsse.CreateEnvelope(controllerRoot)
	require.NoError(t, err)
	state, err := policy.LoadCurrentState(t.Context(), gitRepo, policy.PolicyStagingRef)
	require.NoError(t, err)
	controllerID := "controller-" + base64.URLEncoding.EncodeToString([]byte(controllerLocation))
	state.ControllerMetadata = map[string]*policy.StateMetadata{
		controllerID: {RootEnvelope: controllerEnvelope},
	}
	require.NoError(t, state.Commit(gitRepo, "Add propagated controller metadata", true, false))

	groups, err := repo.ListGlobalRules(t.Context(), policy.PolicyStagingRef)
	require.NoError(t, err)
	require.Len(t, groups, 2)
	require.Equal(t, []globalRule{
		{
			ruleName:     "local-rule",
			ruleType:     tuf.GlobalRuleThresholdType,
			rulePatterns: patterns,
			threshold:    2,
		},
	}, getGlobalRulesForRef(t.Context(), repo, policy.PolicyStagingRef))

	require.NoError(t, repo.RemoveGlobalRule(t.Context(), signer, "local-rule", false, trustpolicyopts.WithRSLEntry()))
	groups, err = repo.ListGlobalRules(t.Context(), policy.PolicyStagingRef)
	require.NoError(t, err)
	require.Len(t, groups, 1)
	require.Equal(t, "controller", groups[0].RepositoryName)
	require.Empty(t, getGlobalRulesForRef(t.Context(), repo, policy.PolicyStagingRef))
}
