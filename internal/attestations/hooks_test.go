// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestSetHookExecutionAttestation(t *testing.T) {
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	policyID := gitinterface.ZeroHash.String()
	stage := tuf.HookStagePreCommit
	hookNames := []string{"hook-name-1"}
	runner := "test-principal"

	hookAttestation := createHookExecutionAttestationEnvelope(t, testRef, testID, policyID, stage, hookNames, runner)

	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	attestations := &Attestations{}

	err := attestations.SetHookExecutionAttestation(repo, hookAttestation, stage.String())
	assert.Nil(t, err)
}

func createHookExecutionAttestationEnvelope(t *testing.T, targetRef, targetID, policyID string, stage tuf.HookStage, hookNames []string, runner string) *sslibdsse.Envelope {
	t.Helper()

	attestation, err := NewHookExecutionAttestation(targetRef, targetID, policyID, stage, hookNames, runner)
	if err != nil {
		t.Fatal(err)
	}
	env, err := dsse.CreateEnvelope(attestation)
	if err != nil {
		t.Fatal(err)
	}

	return env
}
