// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

// SKETCH EXPERIMENT (cedar-integration-sketch, throwaway, not for merge).
// Gap tests in PLAIN gittuf (no Cedar) for the four candidate demo rules.
// Each subtest logs what plain gittuf decides; assertions pin the observed
// behavior so a regression in the experiment is visible.

import (
	"fmt"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	gogithub "github.com/google/go-github/v61/github"
	"github.com/stretchr/testify/require"
)

type sketchPrincipal struct {
	person *tufv02.Person
	signer *ssh.Signer
}

// sketchPrincipals generates fresh ed25519 SSH keys with ssh-keygen and wraps
// each in a v02 Person.
func sketchPrincipals(t *testing.T, names ...string) map[string]*sketchPrincipal {
	t.Helper()
	out := map[string]*sketchPrincipal{}
	dir := t.TempDir()
	for _, name := range names {
		keyPath := filepath.Join(dir, name)
		if output, err := exec.Command("ssh-keygen", "-q", "-t", "ed25519", "-N", "", "-C", name, "-f", keyPath).CombinedOutput(); err != nil {
			t.Fatalf("ssh-keygen: %v %s", err, output)
		}
		signer, err := ssh.NewSignerFromFile(keyPath)
		require.Nil(t, err)
		key := tufv01.NewKeyFromSSLibKey(signer.MetadataKey())
		out[name] = &sketchPrincipal{
			person: &tufv02.Person{PersonID: name, PublicKeys: map[string]*tufv02.Key{key.KeyID: key}},
			signer: signer,
		}
	}
	return out
}

type sketchRule struct {
	name        string
	principals  []string
	patterns    []string
	threshold   int
	terminating bool
}

// sketchState builds a v02 policy State with the given principals and rules in
// the primary rule file. delegations maps a rule name to the rules of a
// delegated rule file with the same name. globalRules are added to root.
func sketchState(t *testing.T, principals map[string]*sketchPrincipal, rules []sketchRule, delegations map[string][]sketchRule, globalRules []tuf.GlobalRule) *State {
	t.Helper()

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	rootKey := tufv01.NewKeyFromSSLibKey(rootSigner.MetadataKey())
	rootMetadata, err := InitializeRootMetadata(rootKey)
	require.Nil(t, err)
	require.Nil(t, rootMetadata.AddPrimaryRuleFilePrincipal(rootKey))
	for _, gr := range globalRules {
		require.Nil(t, rootMetadata.AddGlobalRule(gr))
	}
	rootEnv, err := dsse.CreateEnvelope(rootMetadata)
	require.Nil(t, err)
	rootEnv, err = dsse.SignEnvelope(testCtx, rootEnv, rootSigner)
	require.Nil(t, err)

	buildFile := func(rules []sketchRule) *sslibdsse.Envelope {
		tm := InitializeTargetsMetadata()
		for _, p := range principals {
			require.Nil(t, tm.AddPrincipal(p.person))
		}
		for _, r := range rules {
			require.Nil(t, tm.AddRule(r.name, r.principals, r.patterns, r.threshold))
		}
		v02 := tm.(*tufv02.TargetsMetadata)
		for _, r := range rules {
			if !r.terminating {
				continue
			}
			for _, d := range v02.Delegations.Roles {
				if d.Name == r.name {
					d.Terminating = true
				}
			}
		}
		env, err := dsse.CreateEnvelope(tm)
		require.Nil(t, err)
		env, err = dsse.SignEnvelope(testCtx, env, rootSigner)
		require.Nil(t, err)
		return env
	}

	state := &State{Metadata: &StateMetadata{
		RootEnvelope:        rootEnv,
		TargetsEnvelope:     buildFile(rules),
		DelegationEnvelopes: map[string]*sslibdsse.Envelope{},
	}}
	for name, drules := range delegations {
		state.Metadata.DelegationEnvelopes[name] = buildFile(drules)
	}
	require.Nil(t, state.preprocess())
	return state
}

// sketchAuthorization returns a DSSE envelope signed by the named principals.
// SignatureVerifier.Verify only checks signatures, not the payload, so the
// payload is irrelevant here.
func sketchAuthorization(t *testing.T, principals map[string]*sketchPrincipal, signers ...string) *sslibdsse.Envelope {
	t.Helper()
	env, err := dsse.CreateEnvelope(map[string]string{"sketch": "reference-authorization"})
	require.Nil(t, err)
	for _, s := range signers {
		env, err = dsse.SignEnvelope(testCtx, env, principals[s].signer)
		require.Nil(t, err)
	}
	return env
}

func sketchDecide(t *testing.T, state *State, target string, env *sslibdsse.Envelope, approvers *set.Set[string]) (string, error) {
	t.Helper()
	opts := []verifyGitObjectAndAttestationsOption{}
	if approvers != nil {
		opts = append(opts, withApproverPrincipalIDs(approvers))
	}
	verifiedUsing, _, err := verifyGitObjectAndAttestations(testCtx, state, target, nil, env, opts...)
	return verifiedUsing, err
}

func verdict(verifiedUsing string, err error) string {
	if err != nil {
		return fmt.Sprintf("DENY (%v)", err)
	}
	return fmt.Sprintf("ALLOW (via %q)", verifiedUsing)
}

// Rule 1: forbid a principal on refs/heads/main although a broad
// refs/heads/* rule admits them.
func TestCedarSketchGapRule1(t *testing.T) {
	ps := sketchPrincipals(t, "alice", "bob", "mallory")
	main := "git:refs/heads/main"
	broad := sketchRule{"protect-branches", []string{"alice", "bob", "mallory"}, []string{"git:refs/heads/*"}, 1, false}

	t.Run("naive: specific main rule + broad rule", func(t *testing.T) {
		state := sketchState(t, ps, []sketchRule{
			{"protect-main", []string{"alice", "bob"}, []string{main}, 1, false},
			broad,
		}, nil, nil)
		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "mallory"), nil)
		t.Logf("mallory on main: %s", verdict(v, err))
		require.Nil(t, err)
		require.Equal(t, "protect-branches", v)
	})

	t.Run("main rule terminating, no delegated rule file", func(t *testing.T) {
		state := sketchState(t, ps, []sketchRule{
			{"protect-main", []string{"alice", "bob"}, []string{main}, 1, true},
			broad,
		}, nil, nil)
		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "mallory"), nil)
		t.Logf("mallory on main: %s", verdict(v, err))
		require.Nil(t, err, "terminating is ignored without a delegated rule file")
	})

	t.Run("main rule terminating, with (empty) delegated rule file, ordered first", func(t *testing.T) {
		state := sketchState(t, ps, []sketchRule{
			{"protect-main", []string{"alice", "bob"}, []string{main}, 1, true},
			broad,
		}, map[string][]sketchRule{"protect-main": nil}, nil)
		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "mallory"), nil)
		t.Logf("mallory on main: %s", verdict(v, err))
		require.ErrorIs(t, err, ErrVerifierConditionsUnmet)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "alice"), nil)
		t.Logf("alice on main: %s", verdict(v, err))
		require.Nil(t, err)
		v, err = sketchDecide(t, state, "git:refs/heads/dev", sketchAuthorization(t, ps, "mallory"), nil)
		t.Logf("mallory on dev: %s", verdict(v, err))
		require.Nil(t, err)
	})

	t.Run("same, but broad rule ordered first", func(t *testing.T) {
		state := sketchState(t, ps, []sketchRule{
			broad,
			{"protect-main", []string{"alice", "bob"}, []string{main}, 1, true},
		}, map[string][]sketchRule{"protect-main": nil}, nil)
		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "mallory"), nil)
		t.Logf("mallory on main: %s", verdict(v, err))
		require.Nil(t, err)
	})
}

// Rule 2: threshold k of n plus at least one signer from a named group.
func TestCedarSketchGapRule2(t *testing.T) {
	ps := sketchPrincipals(t, "R", "a", "b", "c", "d", "eve")
	main := "git:refs/heads/main"
	group := []string{"R", "a", "b", "c", "d"}

	t.Run("plain 3-of-5 rule ignores the required signer", func(t *testing.T) {
		state := sketchState(t, ps, []sketchRule{{"protect-main", group, []string{main}, 3, false}}, nil, nil)
		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "a", "b", "c"), nil)
		t.Logf("a,b,c (no R): %s", verdict(v, err))
		require.Nil(t, err)
	})

	t.Run("expansion into one rule per {R,x,y}", func(t *testing.T) {
		others := []string{"a", "b", "c", "d"}
		rules := []sketchRule{}
		for i := 0; i < len(others); i++ {
			for j := i + 1; j < len(others); j++ {
				rules = append(rules, sketchRule{fmt.Sprintf("main-R-%s-%s", others[i], others[j]), []string{"R", others[i], others[j]}, []string{main}, 3, false})
			}
		}
		t.Logf("expansion needs %d rules for n=5, k=3, one required signer", len(rules))
		require.Len(t, rules, 6)
		state := sketchState(t, ps, rules, nil, nil)
		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "a", "b", "c"), nil)
		t.Logf("a,b,c (no R): %s", verdict(v, err))
		require.ErrorIs(t, err, ErrVerifierConditionsUnmet)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "R", "c", "d"), nil)
		t.Logf("R,c,d: %s", verdict(v, err))
		require.Nil(t, err)
	})

	t.Run("brute-force minimum number of threshold rules", func(t *testing.T) {
		// A rule (S, t) is satisfied by any signer set containing t members
		// of S. It is sound for "R and >=3 of group" iff every t-subset of S
		// contains R and t >= 3. Count the minimal satisfying sets each sound
		// rule covers; the minimum cover is the plain-gittuf rule count.
		n := 5 // bit 0 = R
		sound := [][2]int{}
		for s := 1; s < 1<<n; s++ {
			size := popcount(s)
			for th := 1; th <= size; th++ {
				if th >= 3 && size-1 < th && s&1 == 1 {
					// size-1 < th means every th-subset includes R
					sound = append(sound, [2]int{s, th})
				}
			}
		}
		minimalSets := []int{}
		for s := 0; s < 1<<n; s++ {
			if popcount(s) == 3 && s&1 == 1 {
				minimalSets = append(minimalSets, s)
			}
		}
		// Exact minimum set cover over all subsets of sound rules: a minimal
		// set M is covered by rule (S, t) iff |M & S| >= t.
		best := -1
		for combo := 0; combo < 1<<len(sound); combo++ {
			k := popcount(combo)
			if best != -1 && k >= best {
				continue
			}
			ok := true
			for _, m := range minimalSets {
				covered := false
				for i, r := range sound {
					if combo&(1<<i) != 0 && popcount(m&r[0]) >= r[1] {
						covered = true
						break
					}
				}
				if !covered {
					ok = false
					break
				}
			}
			if ok {
				best = k
			}
		}
		t.Logf("sound rules=%d, minimal satisfying sets=%d, exact minimum rules=%d", len(sound), len(minimalSets), best)
		require.Equal(t, 6, best)
	})

	t.Run("2-rule workaround: {R} t=1 + global threshold 3", func(t *testing.T) {
		state := sketchState(t, ps, []sketchRule{
			{"protect-main", []string{"R"}, []string{main}, 1, false},
			{"protect-dev", []string{"eve"}, []string{"git:refs/heads/dev"}, 1, false},
		}, nil, []tuf.GlobalRule{tufv01.NewGlobalRuleThreshold("three-signers-main", []string{main}, 3)})
		v, err := sketchDecide(t, state, main, sketchAuthorization(t, ps, "a", "b", "c"), nil)
		t.Logf("a,b,c (no R): %s", verdict(v, err))
		require.ErrorIs(t, err, ErrVerifierConditionsUnmet)
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "R", "a", "b"), nil)
		t.Logf("R,a,b: %s", verdict(v, err))
		require.Nil(t, err)
		// eve is only trusted for dev, but global rules count every policy
		// principal, so R + a + eve meets "3 signers".
		v, err = sketchDecide(t, state, main, sketchAuthorization(t, ps, "R", "a", "eve"), nil)
		t.Logf("R,a,eve (eve is outside the group): %s", verdict(v, err))
		require.Nil(t, err)
	})
}

// Rules 3 and 4: PR author approves their own PR; required label missing.
// Uses the existing GitHub app fixture: app key = targets1, jane.doe (gpg),
// john.doe (targets2 ssh), protect-main threshold 2, AssociatedIdentities
// {app: "jane.doe"} / {app: "john.doe"}.
func TestCedarSketchGapRules3And4(t *testing.T) {
	repo, state := createTestRepository(t, createTestStateWithThresholdPolicyAndGitHubAppTrust)
	refName := "refs/heads/main"
	fromID := gitinterface.ZeroHash.String()
	treeID := "4b825dc642cb6eb9a060e54bf8d69288fbee4904"
	appSigner := setupSSHKeysForSigning(t, targets1KeyBytes, targets1PubKeyBytes)

	att, err := attestations.LoadCurrentAttestations(repo)
	require.Nil(t, err)

	// Approval attestation from the trusted app listing the PR author.
	approval, err := attestations.NewGitHubPullRequestApprovalAttestation(refName, fromID, treeID, []string{"jane.doe"}, nil)
	require.Nil(t, err)
	approvalEnv, err := dsse.CreateEnvelope(approval)
	require.Nil(t, err)
	approvalEnv, err = dsse.SignEnvelope(testCtx, approvalEnv, appSigner)
	require.Nil(t, err)
	require.Nil(t, att.SetGitHubPullRequestApprovalAttestation(repo, approvalEnv, "https://github.com", 1, tuf.GitHubAppRoleName, refName, fromID, treeID))

	// PR attestation: author jane.doe, no labels. Stored the way
	// addGitHubPullRequestAttestation stores it.
	pr := &gogithub.PullRequest{
		Number: gogithub.Int(7),
		User:   &gogithub.User{Login: gogithub.String("jane.doe"), ID: gogithub.Int64(1)},
		Base:   &gogithub.PullRequestBranch{Ref: gogithub.String("main"), User: &gogithub.User{Login: gogithub.String("octo"), ID: gogithub.Int64(9)}},
	}
	prStmt, err := attestations.NewGitHubPullRequestAttestation("octo", "demo", 7, "1111111111111111111111111111111111111111", pr)
	require.Nil(t, err)
	prEnv, err := dsse.CreateEnvelope(prStmt)
	require.Nil(t, err)
	prEnv, err = dsse.SignEnvelope(testCtx, prEnv, appSigner)
	require.Nil(t, err)
	require.Nil(t, att.SetGitHubPullRequestAuthorization(repo, prEnv, "octo-9/refs/heads/main", "1111111111111111111111111111111111111111"))
	require.Nil(t, att.Commit(repo, "sketch", false, false))

	_, approvers, err := getApproverAttestationAndKeyIDsForIndex(testCtx, repo, state, att, refName, gitinterface.ZeroHash, mustHash(t, treeID), false)
	require.Nil(t, err)
	t.Logf("approver identities loaded by gittuf: %v", approvers.Contents())

	// john.doe signs the reference authorization (merger); jane.doe is the
	// PR author and her own approval is recorded.
	johnSigner := setupSSHKeysForSigning(t, targets2KeyBytes, targets2PubKeyBytes)
	authEnv, err := dsse.CreateEnvelope(map[string]string{"sketch": "reference-authorization"})
	require.Nil(t, err)
	authEnv, err = dsse.SignEnvelope(testCtx, authEnv, johnSigner)
	require.Nil(t, err)

	v, err := sketchDecide(t, state, "git:"+refName, authEnv, approvers)
	t.Logf("rule 3 (author jane.doe approved own PR, john signed): %s", verdict(v, err))
	t.Logf("rule 4 (PR has no labels): same decision; nothing in internal/policy reads the PR attestation")
	require.Nil(t, err)

	v, err = sketchDecide(t, state, "git:"+refName, authEnv, set.NewSet[string]())
	t.Logf("control (no approvals, john only): %s", verdict(v, err))
	require.ErrorIs(t, err, ErrVerifierConditionsUnmet)
}

func popcount(x int) int {
	c := 0
	for ; x != 0; x &= x - 1 {
		c++
	}
	return c
}

func mustHash(t *testing.T, s string) githash.Hash {
	t.Helper()
	h, err := githash.NewHash(s)
	require.Nil(t, err)
	return h
}
