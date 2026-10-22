// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
)

// SKETCH (cedar-integration-sketch, not for merge): tests for the Cedar
// overlay's gate and fail-closed behavior.

const (
	cedarTestPermitAll = `permit (principal, action, resource);`
	cedarTestDenyAll   = cedarTestPermitAll + "\nforbid (principal, action, resource);"
	// Strict validation rejects access to an optional attribute without a
	// `has` guard.
	cedarTestUnguarded = cedarTestPermitAll + "\nforbid (principal, action, resource) when { context.pr.number > 0 };"
	// Validates, but overflows at evaluation time. cedar-go skips policies
	// that error, so without the overlay's check this would allow.
	cedarTestEvalError = cedarTestPermitAll + "\nforbid (principal, action, resource) when { 9223372036854775807 + 1 > 0 };"
)

func TestCedarOverlay(t *testing.T) {
	tests := map[string]struct {
		devMode      bool
		decider      string
		policy       *string // nil: GITTUF_CEDAR_POLICY unset
		policyPath   string  // used instead of writing policy, if set
		authorized   bool
		expectedErr  error
		expectedPass bool
	}{
		"gate off, deny-all policy has no effect": {
			devMode:      false,
			decider:      DeciderCedar,
			policy:       ptr(cedarTestDenyAll),
			authorized:   true,
			expectedPass: true,
		},
		"dev mode without decider, deny-all policy has no effect": {
			devMode:      true,
			decider:      "",
			policy:       ptr(cedarTestDenyAll),
			authorized:   true,
			expectedPass: true,
		},
		"gate on, permit-all allows what native allows": {
			devMode:      true,
			decider:      DeciderCedar,
			policy:       ptr(cedarTestPermitAll),
			authorized:   true,
			expectedPass: true,
		},
		"gate on, permit-all cannot allow what native denies": {
			devMode:     true,
			decider:     DeciderCedar,
			policy:      ptr(cedarTestPermitAll),
			authorized:  false,
			expectedErr: ErrVerificationFailed,
		},
		"gate on, deny-all denies": {
			devMode:     true,
			decider:     DeciderCedar,
			policy:      ptr(cedarTestDenyAll),
			authorized:  true,
			expectedErr: ErrVerificationFailed,
		},
		"gate on, policy not specified denies": {
			devMode:     true,
			decider:     DeciderCedar,
			authorized:  true,
			expectedErr: ErrVerificationFailed,
		},
		"gate on, missing policy file denies": {
			devMode:     true,
			decider:     DeciderCedar,
			policyPath:  filepath.Join(t.TempDir(), "does-not-exist.cedar"),
			authorized:  true,
			expectedErr: ErrVerificationFailed,
		},
		"gate on, unparsable policy denies": {
			devMode:     true,
			decider:     DeciderCedar,
			policy:      ptr("this is not cedar"),
			authorized:  true,
			expectedErr: ErrVerificationFailed,
		},
		"gate on, empty policy denies": {
			devMode:     true,
			decider:     DeciderCedar,
			policy:      ptr(""),
			authorized:  true,
			expectedErr: ErrVerificationFailed,
		},
		"gate on, policy failing strict validation denies": {
			devMode:     true,
			decider:     DeciderCedar,
			policy:      ptr(cedarTestUnguarded),
			authorized:  true,
			expectedErr: ErrVerificationFailed,
		},
		"gate on, evaluation error denies": {
			devMode:     true,
			decider:     DeciderCedar,
			policy:      ptr(cedarTestEvalError),
			authorized:  true,
			expectedErr: ErrVerificationFailed,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(cedarDecisionLogKey, "")
			if test.devMode {
				t.Setenv(dev.DevModeKey, "1")
			} else {
				t.Setenv(dev.DevModeKey, "")
			}
			t.Setenv(DeciderKey, test.decider)
			t.Setenv(CedarPolicyKey, "")
			switch {
			case test.policyPath != "":
				t.Setenv(CedarPolicyKey, test.policyPath)
			case test.policy != nil:
				t.Setenv(CedarPolicyKey, writeCedarTestPolicy(t, *test.policy))
			}

			repo, _ := createTestRepository(t, createTestStateWithPolicy)
			refName := "refs/heads/main"
			signingKey := gpgKeyBytes
			if !test.authorized {
				signingKey = gpgUnauthorizedKeyBytes
			}
			commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, signingKey)
			entry := rsl.NewReferenceEntry(refName, commitIDs[0])
			common.CreateTestRSLReferenceEntryCommit(t, repo, entry, signingKey)

			_, err := NewPolicyVerifier(repo).VerifyRef(testCtx, refName)
			if test.expectedPass {
				assert.Nil(t, err)
			} else {
				assert.ErrorIs(t, err, test.expectedErr)
			}
		})
	}
}

func TestVerifyCedarOverlayErrors(t *testing.T) {
	t.Setenv(cedarDecisionLogKey, "")
	t.Setenv(dev.DevModeKey, "1")
	t.Setenv(DeciderKey, DeciderCedar)

	_, state := createTestRepository(t, createTestStateWithPolicy)

	tests := map[string]struct {
		policy      *string
		expectedErr error
	}{
		"not specified":       {policy: nil, expectedErr: ErrCedarPolicyNotSpecified},
		"unparsable":          {policy: ptr("permit"), expectedErr: ErrInvalidCedarPolicy},
		"empty":               {policy: ptr(""), expectedErr: ErrInvalidCedarPolicy},
		"strict validation":   {policy: ptr(cedarTestUnguarded), expectedErr: ErrInvalidCedarPolicy},
		"deny-all":            {policy: ptr(cedarTestDenyAll), expectedErr: ErrCedarDenied},
		"evaluation error":    {policy: ptr(cedarTestEvalError), expectedErr: ErrCedarDenied},
		"permit-all, allowed": {policy: ptr(cedarTestPermitAll)},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Setenv(CedarPolicyKey, "")
			if test.policy != nil {
				t.Setenv(CedarPolicyKey, writeCedarTestPolicy(t, *test.policy))
			}

			// gitID is nil as in verify-mergeable: only the policy and facts
			// matter here.
			err := verifyCedarOverlay(state, "git:refs/heads/main", nil, "protect-main", &nativeDecisionFacts{}, &verifyGitObjectAndAttestationsOptions{})
			if test.expectedErr == nil {
				assert.Nil(t, err)
				return
			}
			assert.ErrorIs(t, err, test.expectedErr)
			if errors.Is(test.expectedErr, ErrCedarDenied) {
				assert.ErrorIs(t, err, ErrVerifierConditionsUnmet)
			}
		})
	}
}

func TestCedarOverlaySkipsFileTargets(t *testing.T) {
	t.Setenv(cedarDecisionLogKey, "")
	t.Setenv(dev.DevModeKey, "1")
	t.Setenv(DeciderKey, DeciderCedar)
	t.Setenv(CedarPolicyKey, writeCedarTestPolicy(t, cedarTestDenyAll))

	repo, state := createTestRepository(t, createTestStateWithPolicy)
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, "refs/heads/main", 1, gpgKeyBytes)

	// file: targets are out of scope for the overlay, so the deny-all policy
	// is not consulted.
	_, _, err := verifyGitObjectAndAttestations(testCtx, state, "file:1", commitIDs[0], nil)
	assert.Nil(t, err)

	_, _, err = verifyGitObjectAndAttestations(testCtx, state, "git:refs/heads/main", commitIDs[0], nil)
	assert.ErrorIs(t, err, ErrCedarDenied)
}

func writeCedarTestPolicy(t *testing.T, policy string) string {
	t.Helper()

	path := filepath.Join(t.TempDir(), "policy.cedar")
	if err := os.WriteFile(path, []byte(policy), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func ptr[T any](v T) *T {
	return &v
}
