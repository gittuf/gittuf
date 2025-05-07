// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v01

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/tuf"
	ita "github.com/in-toto/attestation/go/v1"
	"github.com/stretchr/testify/assert"
)

func TestNewHookExecutionAttestationForStage(t *testing.T) {
	targetRef := "refs/heads/main"
	stage := tuf.HookStagePreCommit
	testID := gitinterface.ZeroHash.String()
	hookNames := []string{"test-hook"}
	principalName := "principal"

	attestation, err := NewHookExecutionAttestationForStage(targetRef, testID, testID, stage, hookNames, principalName)
	assert.Nil(t, err)

	// Check value of statement type
	assert.Equal(t, ita.StatementTypeUri, attestation.Type)

	// Check subject contents
	assert.Equal(t, 1, len(attestation.Subject))
	assert.Contains(t, attestation.Subject[0].Digest, digestGitTreeKey)
	assert.Equal(t, attestation.Subject[0].Digest[digestGitTreeKey], testID)

	// Check predicate type
	assert.Equal(t, PredicateType, attestation.PredicateType)

	// Check predicate
	predicate := attestation.Predicate.AsMap()
	assert.Equal(t, targetRef, predicate[targetRefKey])
	assert.Equal(t, testID, predicate[targetIDKey])
	assert.Equal(t, testID, predicate[policyEntryKey])
	assert.Equal(t, stage.String(), predicate[stageKey])
	assert.Len(t, predicate[hookNamesKey], 1)
	assert.Equal(t, principalName, predicate[runnerKey])
}
