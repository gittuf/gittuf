// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/rsl"
	gogithub "github.com/google/go-github/v61/github"
	"github.com/stretchr/testify/require"
)

// SKETCH (cedar-integration-sketch, not for merge): milestone 3 demo. The PR
// author approving their own PR, run three ways: plain gittuf, Cedar with a
// permits-only policy, and Cedar with the self-approval ban. The approval is a
// real GitHub app approval attestation read by gittuf; the pull request
// attestation is hand-recorded and passed via GITTUF_CEDAR_PR_ATTESTATION.

const (
	demoAppIdentityJane = "jane+1001"
	demoAppIdentityJohn = "john+1002"
)

// createTestStateForCedarDemoRule3 is createTestStateWithThresholdPolicyAndGitHubAppTrust
// with associated identities in the format the gittuf GitHub app records
// ("<login>+<id>").
func createTestStateForCedarDemoRule3(t *testing.T) *State {
	t.Helper()

	state := createTestStateWithThresholdPolicyAndGitHubAppTrust(t)

	targetsMetadata, err := state.GetTargetsMetadata(TargetsRoleName, false)
	require.Nil(t, err)
	identities := map[string]string{"jane.doe": demoAppIdentityJane, "john.doe": demoAppIdentityJohn}
	for principalID, principal := range targetsMetadata.GetPrincipals() {
		person, isPerson := principal.(*tufv02.Person)
		if !isPerson {
			continue
		}
		if identity, has := identities[principalID]; has {
			person.AssociatedIdentities = map[string]string{tuf.GitHubAppRoleName: identity}
		}
	}

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	targetsEnv, err := dsse.CreateEnvelope(targetsMetadata)
	require.Nil(t, err)
	targetsEnv, err = dsse.SignEnvelope(testCtx, targetsEnv, signer)
	require.Nil(t, err)
	state.Metadata.TargetsEnvelope = targetsEnv
	require.Nil(t, state.preprocess())

	return state
}

func TestCedarSketchDemoRule3(t *testing.T) {
	t.Setenv(cedarDecisionLogKey, "")

	policyDir := filepath.Join("..", "..", "demo", "cedar")
	permitAll := filepath.Join(policyDir, "permit-all.cedar")
	withRule := filepath.Join(t.TempDir(), "with-rule3.cedar")
	concatenateFiles(t, withRule, permitAll, filepath.Join(policyDir, "rule3-no-self-approval.cedar"))

	tests := map[string]struct {
		authorLogin string
		authorID    int64
		expected    [3]bool // plain, permits-only, with rule
	}{
		"violating: author jane approved her own PR": {
			authorLogin: "jane", authorID: 1001,
			expected: [3]bool{true, true, false},
		},
		"compliant: external author, jane approved": {
			authorLogin: "ghost", authorID: 9999,
			expected: [3]bool{true, true, true},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			repo, _ := createTestRepository(t, createTestStateForCedarDemoRule3)
			refName := "refs/heads/main"

			// The commit adds file "1", which the fixture's file rule trusts
			// only jane's GPG key for, so jane's key signs the commit. john.doe
			// (targets2 key) signs the RSL entry for main.
			commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
			treeID, err := repo.GetCommitTreeID(commitIDs[0])
			require.Nil(t, err)

			// The trusted GitHub app (targets1 key) records jane's approval.
			approval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, gitinterface.ZeroHash.String(), treeID.String(), []string{demoAppIdentityJane}, nil)
			require.Nil(t, err)
			approvalEnv, err := dsse.CreateEnvelope(approval)
			require.Nil(t, err)
			approvalEnv, err = dsse.SignEnvelope(testCtx, approvalEnv, setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes))
			require.Nil(t, err)
			currentAttestations, err := attestations.LoadCurrentAttestations(repo)
			require.Nil(t, err)
			require.Nil(t, currentAttestations.SetGitHubPullRequestApprovalAttestation(repo, approvalEnv, "https://github.com", 1, tuf.GitHubAppRoleName, refName, gitinterface.ZeroHash.String(), treeID.String()))
			require.Nil(t, currentAttestations.Commit(repo, "Add GitHub pull request approval", true, false))

			entry := rsl.NewReferenceEntry(refName, commitIDs[0])
			common.CreateTestRSLReferenceEntryCommit(t, repo, entry, targets2KeyBytes)

			// SKETCH: hand-recorded pull request attestation, in the format
			// created by `gittuf attest github pull-request`.
			pullRequest := &gogithub.PullRequest{
				Number: gogithub.Int(7),
				User:   &gogithub.User{Login: gogithub.String(test.authorLogin), ID: gogithub.Int64(test.authorID)},
				Base:   &gogithub.PullRequestBranch{Ref: gogithub.String("main")},
			}
			statement, err := attestations.NewGitHubPullRequestAttestation("octo", "demo", 7, commitIDs[0].String(), pullRequest)
			require.Nil(t, err)
			statementBytes, err := json.Marshal(statement)
			require.Nil(t, err)
			prPath := filepath.Join(t.TempDir(), "pull-request.json")
			require.Nil(t, os.WriteFile(prPath, statementBytes, 0o600))
			t.Setenv(CedarPullRequestAttestationKey, prPath)

			t.Logf("%s: author %s+%d, approver %s, RSL entry signed by john", name, test.authorLogin, test.authorID, demoAppIdentityJane)
			runs := []struct {
				label  string
				policy string
			}{
				{"1. plain gittuf", ""},
				{"2. Cedar, permits-only policy", permitAll},
				{"3. Cedar, permit-all + rule3-no-self-approval.cedar", withRule},
			}
			for i, run := range runs {
				decisions := filepath.Join(t.TempDir(), "decisions.tsv")
				if run.policy == "" {
					t.Setenv(dev.DevModeKey, "")
					t.Setenv(DeciderKey, "")
				} else {
					t.Setenv(dev.DevModeKey, "1")
					t.Setenv(DeciderKey, DeciderCedar)
					t.Setenv(CedarPolicyKey, run.policy)
					t.Setenv(cedarDecisionLogKey, decisions)
				}

				_, err := NewPolicyVerifier(repo).VerifyRef(testCtx, refName)
				got := err == nil
				t.Logf("   %-58s %-4s (expected %s)", run.label, passFail(got), passFail(test.expected[i]))
				if !got {
					if logBytes, readErr := os.ReadFile(decisions); readErr == nil {
						for _, line := range strings.Split(strings.TrimSpace(string(logBytes)), "\n") {
							if strings.HasPrefix(line, "deny") {
								t.Logf("      cedar: %s", strings.ReplaceAll(line, "\t", " "))
							}
						}
					}
				}
				require.Equal(t, test.expected[i], got, run.label)
			}
		})
	}
}

func concatenateFiles(t *testing.T, out string, paths ...string) {
	t.Helper()

	contents := []byte{}
	for _, path := range paths {
		fileBytes, err := os.ReadFile(path)
		require.Nil(t, err)
		contents = append(contents, fileBytes...)
		contents = append(contents, '\n')
	}
	require.Nil(t, os.WriteFile(out, contents, 0o600)) //nolint:gosec // test temp file
}

func passFail(pass bool) string {
	if pass {
		return "PASS"
	}
	return "FAIL"
}
