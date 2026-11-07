// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

//go:build cedarsketchbench

package policy

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/cedar-policy/cedar-go"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/pkg/rsl"
)

// SKETCH (cedar-integration-sketch, not for merge): overhead of the Cedar
// overlay in cedar.go. Run with:
//
//	go test -tags cedarsketchbench -run TestCedarSketchOverheadBench -v ./internal/policy/
//
// Knobs: SKETCH_ENTRIES (default 300), SKETCH_ROUNDS (default 3).

// cedarSketchBenchPolicy is the "realistic" policy: permit-all plus four
// forbid rules. Every forbid is evaluated on every decision but none fires in
// the bench repository.
//
// The bench repository's principals are tufv01 keys, so they belong to no
// Cedar group and creditedGroups is always empty. The two group rules would
// therefore deny every entry if scoped to main. Instead of scoping them to a
// ref that does not match up front (which would let Cedar skip the group
// predicate after one string compare), we keep the group predicate FIRST,
// where it evaluates to true, and put the ref check
// `resource.name == "refs/heads/does-not-exist"` LAST, where it evaluates to
// false. Cedar's && short-circuits left to right, so the group predicate and
// one string compare are both evaluated, the same work the real rule
// (`resource.name == "refs/heads/main" && !...`) does on main.
//
// The self-approval and mallory rules are scoped to main as written: they do
// not fire naturally (there is no PR fact, and mallory is not a principal).
const cedarSketchBenchPolicy = `// Baseline: allow every change gittuf's native decision accepted.
permit (principal, action, resource);

// Required group on main (neutralized, see the bench file comment).
// Real rule: resource.name == "refs/heads/main" &&
//            !context.creditedGroups.contains(Gittuf::Group::"release-managers")
forbid (principal, action in [Gittuf::Action::"updateRef", Gittuf::Action::"updateTag"], resource)
when {
  !context.creditedGroups.contains(Gittuf::Group::"release-managers") &&
  resource.name == "refs/heads/does-not-exist"
};

// No self-approval on main: the PR author's approval doesn't count.
forbid (principal, action in [Gittuf::Action::"updateRef", Gittuf::Action::"updateTag"], resource)
when {
  resource.name == "refs/heads/main" &&
  context has pr &&
  context.pr has author &&
  context.approvers.contains(context.pr.author)
};

// Separation of duties (neutralized, see the bench file comment).
// Real rule: resource.name == "refs/heads/main" && !containsAll(...)
forbid (principal, action in [Gittuf::Action::"updateRef", Gittuf::Action::"updateTag"], resource)
when {
  !context.creditedGroups.containsAll([
    Gittuf::Group::"developers",
    Gittuf::Group::"security",
    Gittuf::Group::"release-managers"
  ]) &&
  resource.name == "refs/heads/does-not-exist"
};

// Mallory may never be credited on main.
forbid (principal, action in [Gittuf::Action::"updateRef", Gittuf::Action::"updateTag"], resource)
when {
  resource.name == "refs/heads/main" &&
  context.credited.contains(Gittuf::Principal::"mallory")
};
`

const cedarSketchDenyAllPolicy = `permit (principal, action, resource);
forbid (principal, action, resource);
`

func TestCedarSketchOverheadBench(t *testing.T) {
	n := cedarSketchEnvInt(t, "SKETCH_ENTRIES", 300)
	rounds := cedarSketchEnvInt(t, "SKETCH_ROUNDS", 3)

	policyDir := t.TempDir()
	realisticPath := filepath.Join(policyDir, "realistic.cedar")
	denyAllPath := filepath.Join(policyDir, "deny-all.cedar")
	if err := os.WriteFile(realisticPath, []byte(cedarSketchBenchPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(denyAllPath, []byte(cedarSketchDenyAllPolicy), 0o600); err != nil {
		t.Fatal(err)
	}
	// Make sure no evidence log or PR attestation leaks in from the caller.
	t.Setenv(cedarDecisionLogKey, "")
	t.Setenv(CedarPullRequestAttestationKey, "")

	repo, state := createTestRepository(t, createTestStateWithPolicy)
	refName := "refs/heads/main"
	target := "git:" + refName
	start := time.Now()
	entries := make([]*rsl.ReferenceEntry, 0, n)
	for i := 0; i < n; i++ {
		commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repo, refName, 1, gpgKeyBytes)
		entry := rsl.NewReferenceEntry(refName, commitIDs[0])
		entry.ID = common.CreateTestRSLReferenceEntryCommit(t, repo, entry, gpgKeyBytes)
		entries = append(entries, entry)
	}
	t.Logf("setup: %d RSL entries for %s in %v (rounds=%d)", n, refName, time.Since(start), rounds)

	const (
		modeNative     = "native"
		modeExhaustive = "cedar-exhaustive"
		modeReuse      = "cedar-reuse"
	)
	modes := []string{modeNative, modeExhaustive, modeReuse}
	setMode := func(mode string) {
		switch mode {
		case modeNative:
			t.Setenv(dev.DevModeKey, "")
			t.Setenv(DeciderKey, "")
			t.Setenv(cedarSignersKey, "")
		case modeExhaustive:
			t.Setenv(dev.DevModeKey, "1")
			t.Setenv(DeciderKey, DeciderCedar)
			t.Setenv(cedarSignersKey, cedarSignersExhaustive)
		case modeReuse:
			t.Setenv(dev.DevModeKey, "1")
			t.Setenv(DeciderKey, DeciderCedar)
			t.Setenv(cedarSignersKey, "reuse")
		default:
			t.Fatalf("unknown mode %q", mode)
		}
	}
	t.Setenv(CedarPolicyKey, realisticPath)

	// The realistic policy must pass the strict validator in cedar.go.
	policySet, err := loadCedarPolicySet()
	if err != nil {
		t.Fatalf("realistic policy rejected: %v", err)
	}
	policyCount := 0
	for range policySet.All() {
		policyCount++
	}
	t.Logf("realistic policy: %d policies, strict validation OK", policyCount)

	// Sanity: every mode verifies the ref end to end and decides every entry.
	for _, mode := range modes {
		setMode(mode)
		if _, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, refName); err != nil {
			t.Fatalf("sanity: mode %s VerifyRefFull: %v", mode, err)
		}
		for _, entry := range entries {
			if _, _, err := verifyGitObjectAndAttestations(testCtx, state, target, entry.ID, nil); err != nil {
				t.Fatalf("sanity: mode %s decision for %s: %v", mode, entry.ID, err)
			}
		}
		t.Logf("sanity: mode %-16s VerifyRefFull OK, %d decisions OK", mode, n)
	}

	// Sanity: a deny-all policy makes the gate-on modes fail (and leaves
	// native unaffected). VerifyRefFull doesn't wrap the decision error, so
	// the Cedar decision log is used to confirm the failure is a Cedar denial,
	// and a direct decision call must return ErrCedarDenied.
	t.Setenv(CedarPolicyKey, denyAllPath)
	for _, mode := range modes {
		setMode(mode)
		logPath := filepath.Join(policyDir, "deny-all-"+mode+".log")
		t.Setenv(cedarDecisionLogKey, logPath)
		_, verifyErr := NewPolicyVerifier(repo).VerifyRefFull(testCtx, refName)
		_, _, decisionErr := verifyGitObjectAndAttestations(testCtx, state, target, entries[0].ID, nil)
		t.Setenv(cedarDecisionLogKey, "")
		logBytes, _ := os.ReadFile(logPath) //nolint:gosec
		denials := strings.Count(string(logBytes), "deny\t")
		switch {
		case mode == modeNative && (verifyErr != nil || decisionErr != nil || len(logBytes) != 0):
			t.Fatalf("sanity: deny-all policy affected native mode: verify=%v decision=%v log=%q", verifyErr, decisionErr, logBytes)
		case mode == modeNative:
			t.Logf("sanity: deny-all mode %-16s VerifyRefFull OK, decision OK, no Cedar log lines (gate off)", mode)
		case verifyErr == nil:
			t.Fatalf("sanity: deny-all policy did not fail VerifyRefFull in mode %s", mode)
		case !errors.Is(decisionErr, ErrCedarDenied):
			t.Fatalf("sanity: deny-all mode %s decision error is not ErrCedarDenied: %v", mode, decisionErr)
		case denials == 0:
			t.Fatalf("sanity: deny-all mode %s failed without a Cedar denial: %v (log %q)", mode, verifyErr, logBytes)
		default:
			t.Logf("sanity: deny-all mode %-16s VerifyRefFull failed as expected (%v); %d Cedar deny log lines; decision err: %v", mode, verifyErr, denials, decisionErr)
		}
	}
	t.Setenv(CedarPolicyKey, realisticPath)

	type result struct {
		all  []time.Duration
		best time.Duration
	}
	record := func(results map[string]*result, mode string, d time.Duration) {
		r, has := results[mode]
		if !has {
			r = &result{}
			results[mode] = r
		}
		r.all = append(r.all, d)
		if len(r.all) == 1 || d < r.best {
			r.best = d
		}
	}
	report := func(label string, r testing.BenchmarkResult, perOps int) time.Duration {
		per := time.Duration(r.NsPerOp())
		extra := ""
		if perOps > 0 {
			extra = fmt.Sprintf("  (%.1f µs per decision)", float64(r.NsPerOp())/float64(perOps)/1000)
		}
		t.Logf("%-48s %5d iters %14v/op %9d allocs/op%s", label, r.N, per, r.AllocsPerOp(), extra)
		return per
	}

	// End to end: VerifyRefFull over n entries. A fresh PolicyVerifier each
	// iteration, so policy states and verifier caches are rebuilt.
	e2e := map[string]*result{}
	for round := 1; round <= rounds; round++ {
		for _, mode := range modes {
			setMode(mode)
			r := testing.Benchmark(func(b *testing.B) {
				for b.Loop() {
					if _, err := NewPolicyVerifier(repo).VerifyRefFull(testCtx, refName); err != nil {
						b.Fatal(err)
					}
				}
			})
			if r.N == 0 {
				t.Fatalf("e2e mode %s round %d: benchmark failed", mode, round)
			}
			record(e2e, mode, report(fmt.Sprintf("e2e VerifyRefFull round%d mode=%s", round, mode), r, 0))
		}
	}

	// Decision only: one wrapper call per entry against the loaded state.
	dec := map[string]*result{}
	for round := 1; round <= rounds; round++ {
		for _, mode := range modes {
			setMode(mode)
			r := testing.Benchmark(func(b *testing.B) {
				for b.Loop() {
					for _, entry := range entries {
						if _, _, err := verifyGitObjectAndAttestations(testCtx, state, target, entry.ID, nil); err != nil {
							b.Fatal(err)
						}
					}
				}
			})
			if r.N == 0 {
				t.Fatalf("decision mode %s round %d: benchmark failed", mode, round)
			}
			record(dec, mode, report(fmt.Sprintf("decision x%d round%d mode=%s", n, round, mode), r, n))
		}
	}

	// Cedar pieces alone, on the request the overlay builds for the first
	// entry in reuse mode (credited = the rule's accepted signer).
	setMode(modeReuse)
	facts := &nativeDecisionFacts{}
	verifiedUsing, _, err := verifyGitObjectAndAttestationsNative(testCtx, state, target, entries[0].ID, nil, withNativeFacts(facts))
	if err != nil {
		t.Fatal(err)
	}
	if facts.acceptedPrincipalIDs == nil {
		facts.acceptedPrincipalIDs = set.NewSet[string]()
	}
	options := &verifyGitObjectAndAttestationsOptions{}
	request, entities, err := buildCedarRequest(state, target, entries[0].ID, verifiedUsing, facts, options)
	if err != nil {
		t.Fatal(err)
	}
	if decision, diagnostic := cedar.Authorize(policySet, entities, request); decision != cedar.Allow || len(diagnostic.Errors) != 0 {
		t.Fatalf("prebuilt request not allowed: %v %+v", decision, diagnostic)
	}
	t.Logf("prebuilt request: verifiedUsing=%q credited=%v", verifiedUsing, facts.acceptedPrincipalIDs.Contents())
	r := testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			_, _ = cedar.Authorize(policySet, entities, request)
		}
	})
	authorizeOnly := report(fmt.Sprintf("cedar.Authorize only (%d policies)", policyCount), r, 0)
	r = testing.Benchmark(func(b *testing.B) {
		for b.Loop() {
			if _, _, err := buildCedarRequest(state, target, entries[0].ID, verifiedUsing, facts, options); err != nil {
				b.Fatal(err)
			}
		}
	})
	buildOnly := report("buildCedarRequest only", r, 0)

	pct := func(a, b time.Duration) float64 { return 100 * float64(a-b) / float64(b) }
	span := func(r *result) string {
		lo, hi := r.all[0], r.all[0]
		parts := make([]string, 0, len(r.all))
		for _, d := range r.all {
			lo, hi = min(lo, d), max(hi, d)
			parts = append(parts, d.String())
		}
		return fmt.Sprintf("rounds=[%s] range=[%v .. %v]", strings.Join(parts, ", "), lo, hi)
	}
	perDecision := func(d time.Duration) float64 { return float64(d.Nanoseconds()) / float64(n) / 1000 }

	t.Logf("SUMMARY n=%d rounds=%d (best of rounds)", n, rounds)
	for _, mode := range modes {
		t.Logf("  e2e      %-16s best=%v %s", mode, e2e[mode].best, span(e2e[mode]))
	}
	for _, mode := range modes {
		t.Logf("  decision %-16s best=%v (%.1f µs/decision) %s", mode, dec[mode].best, perDecision(dec[mode].best), span(dec[mode]))
	}
	for _, mode := range []string{modeExhaustive, modeReuse} {
		t.Logf("  overhead %-16s vs native: e2e %+.1f%% | decision %+.1f%% (%+.1f µs/decision)", mode,
			pct(e2e[mode].best, e2e[modeNative].best),
			pct(dec[mode].best, dec[modeNative].best),
			perDecision(dec[mode].best)-perDecision(dec[modeNative].best))
	}
	t.Logf("  cedar.Authorize only: %v/call | buildCedarRequest only: %v/call", authorizeOnly, buildOnly)
}

func cedarSketchEnvInt(t *testing.T, key string, def int) int {
	t.Helper()
	s := os.Getenv(key)
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		t.Fatalf("invalid %s=%q", key, s)
	}
	return v
}
