// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
	"time"
)

// PrintFinalReport emits a structured, human-readable findings report
// suitable for sharing with the gittuf maintainers (Paulo Gomes et al.)
// as the deliverable from the GAP-1 PoC.
func PrintFinalReport(
	sha1State *SHA1RepoState,
	migration *MigrationResult,
	resultA, resultB, resultC *ExperimentResult,
) {
	sep := strings.Repeat("=", 60)
	fmt.Println(sep)
	fmt.Println("  GAP-1 HASH AGILITY PoC — FINAL FINDINGS REPORT")
	fmt.Printf("  Generated: %s\n", time.Now().UTC().Format(time.RFC3339))
	fmt.Println(sep)
	fmt.Println()

	// ── Baseline ─────────────────────────────────────────────────────────
	fmt.Println("BASELINE (SHA-1 Repository)")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("  Repo path:       %s\n", sha1State.RepoPath)
	fmt.Printf("  Commits:         %d\n", len(sha1State.CommitSHA1s))
	fmt.Printf("  RSL entries:     %d\n", sha1State.RSLEntryCount)
	fmt.Printf("  Final HEAD:      %s\n", sha1State.FinalHeadSHA1)
	fmt.Printf("  Final RSL tip:   %s\n", sha1State.FinalRSLTipSHA1)
	fmt.Println()

	// ── Migration ──────────────────────────────────────────────────────────
	fmt.Println("MIGRATION (SHA-256 Repository)")
	fmt.Println(strings.Repeat("-", 40))
	fmt.Printf("  SHA-256 repo:    %s\n", migration.SHA256RepoPath)
	fmt.Printf("  New HEAD:        %s\n", migration.NewHeadSHA256)
	fmt.Printf("  Objects mapped:  %d SHA-1 -> SHA-256 translations\n", len(migration.SHA1ToSHA256))
	fmt.Printf("  verify-ref err:  %s\n", migration.VerifyError)
	fmt.Println()

	// ── Experiment A ───────────────────────────────────────────────────────
	fmt.Println("EXPERIMENT A — In-Memory Hash Translation Layer")
	fmt.Println(strings.Repeat("-", 40))
	passA, failA := countFindings(resultA)
	fmt.Printf("  Pass: %d | Fail: %d\n", passA, failA)
	for _, f := range resultA.Findings {
		status := "[PASS]"
		if !f.Passed {
			status = "[FAIL]"
		}
		fmt.Printf("  %s %s\n", status, f.Description)
	}
	fmt.Printf("\n  VERDICT: %s\n\n", wrap(resultA.Verdict, 54, "           "))

	// ── Experiment B ───────────────────────────────────────────────────────
	fmt.Println("EXPERIMENT B — Snapshot + Fresh Start")
	fmt.Println(strings.Repeat("-", 40))
	passB, failB := countFindings(resultB)
	fmt.Printf("  Pass: %d | Fail: %d\n", passB, failB)
	for _, f := range resultB.Findings {
		status := "[PASS]"
		if !f.Passed {
			status = "[FAIL]"
		}
		fmt.Printf("  %s %s\n", status, f.Description)
	}
	if resultB.SnapshotManifest != nil {
		m := resultB.SnapshotManifest
		fmt.Println()
		fmt.Println("  Snapshot Manifest:")
		fmt.Printf("    schema_version:  %s\n", m.SchemaVersion)
		fmt.Printf("    frozen_at:       %s\n", m.FrozenAt.Format(time.RFC3339))
		fmt.Printf("    sha1_repo_head:  %s\n", m.SHA1RepoHead)
		fmt.Printf("    rsl_tip:         %s\n", m.RSLTip)
		fmt.Printf("    rsl_entries:     %d\n", m.RSLEntryCount)
		fmt.Printf("    chain_hash:      %s\n", m.RSLChainHash)
	}
	fmt.Printf("\n  VERDICT: %s\n\n", wrap(resultB.Verdict, 54, "           "))

	// ── Experiment C ───────────────────────────────────────────────────────
	fmt.Println("EXPERIMENT C — Cross-Signing Attestations (in-toto)")
	fmt.Println(strings.Repeat("-", 40))
	passC, failC := countFindings(resultC)
	fmt.Printf("  Pass: %d | Fail: %d\n", passC, failC)
	for _, f := range resultC.Findings {
		status := "[PASS]"
		if !f.Passed {
			status = "[FAIL]"
		}
		fmt.Printf("  %s %s\n", status, f.Description)
	}
	fmt.Printf("\n  VERDICT: %s\n\n", wrap(resultC.Verdict, 54, "           "))

	// ── Code-Level Findings ────────────────────────────────────────────────
	fmt.Println(sep)
	fmt.Println("CODE-LEVEL FINDINGS (from codebase analysis)")
	fmt.Println(sep)
	codeFindings := []struct {
		File    string
		Finding string
		Impact  string
	}{
		{
			File:    "pkg/gitinterface/hash.go:52",
			Finding: "ZeroHash is SHA-1 only (20 zero bytes). TODO already exists.",
			Impact:  "LOW — Fix: detect repo object format and use correct zero hash.",
		},
		{
			File:    "pkg/gitinterface/hash.go:57",
			Finding: "NewHash() already validates both SHA-1 (40 hex) and SHA-256 (64 hex).",
			Impact:  "GOOD — The Hash type is already forward-compatible.",
		},
		{
			File:    "internal/rsl/rsl.go:191",
			Finding: "RSL commit messages store TargetID as plain hex (algorithm-agnostic).",
			Impact:  "NEUTRAL — The string is agnostic but there is no algorithm tag stored.",
		},
		{
			File:    "pkg/gitinterface/commit.go:67-127",
			Finding: "CommitUsingSpecificKey uses go-git plumbing (SHA-1 only in v5.x).",
			Impact:  "HIGH — Will not work in SHA-256 repos. go-git v6 or CLI needed for SHA-256 signing.",
		},
		{
			File:    "internal/attestations/authorization.go:140-142",
			Finding: "ReferenceAuthorizationPath() encodes SHA-1 hex directly in Git tree paths.",
			Impact:  "HIGH — Switching to SHA-256 changes path structure, breaks lookups.",
		},
		{
			File:    "internal/rsl/rsl.go (all entry types)",
			Finding: "RSL entries are signed git commits. Their messages cannot be altered.",
			Impact:  "CRITICAL — Makes Approach A (translation) fundamentally impossible.",
		},
	}

	for _, cf := range codeFindings {
		fmt.Printf("  File: %s\n", cf.File)
		fmt.Printf("  Finding: %s\n", cf.Finding)
		fmt.Printf("  Impact:  %s\n\n", cf.Impact)
	}

	// ── Recommendation ─────────────────────────────────────────────────────
	fmt.Println(sep)
	fmt.Println("RECOMMENDATION & SYNTHESIS")
	fmt.Println(sep)
	rec := "\n" +
		"  We recommend APPROACH B (Snapshot + Fresh Start) as primary,\n" +
		"  augmented by APPROACH C (Cross-Signing Attestations) as a bridge:\n" +
		"\n" +
		"  1. SIGNATURES ARE INVIOLABLE (Rejecting Approach A)\n" +
		"     RSL entries are signed Git commits. Embedding SHA-1 hashes in\n" +
		"     signed messages means in-memory translation cannot rewrite them\n" +
		"     without breaking signatures. Approach A fails cryptographically.\n" +
		"\n" +
		"  2. PRIMARY STRATEGY: Snapshot + Fresh Start (Approach B)\n" +
		"     Freeze the SHA-1 repo, capture a deterministic Merkle-style root\n" +
		"     over the RSL chain in a SnapshotManifest, anchor to Rekor/Sigstore,\n" +
		"     and initialize fresh gittuf metadata in the SHA-256 repo.\n" +
		"\n" +
		"  3. OPTIONAL PROVENANCE BRIDGE: Cross-Signing Attestations (Approach C)\n" +
		"     For repositories requiring continuous verification across the\n" +
		"     migration boundary, maintainers can issue in-toto DSSE statements\n" +
		"     certifying hash equivalence without modifying historical commits.\n" +
		"\n" +
		"  REQUIRED CHANGES FOR PRODUCTION:\n" +
		"  a. Implement SnapshotManifest generation + Rekor submission\n" +
		"  b. Fix ZeroHash sentinel for SHA-256 repos\n" +
		"  c. Upgrade go-git to v6 or use Git CLI fallback for SHA-256 signing\n" +
		"  d. Update ReferenceAuthorizationPath() path length logic\n" +
		"  e. Add `gittuf migrate sha256` CLI subcommand\n" +
		"\n" +
		"  Next step: Present this PoC + findings to Paulo Gomes & Patrick Zielinski.\n"
	fmt.Println(rec)
}

func countFindings(r *ExperimentResult) (pass, fail int) {
	for _, f := range r.Findings {
		if f.Passed {
			pass++
		} else {
			fail++
		}
	}
	return
}

// wrap wraps long strings at the given width, indenting continuation lines.
func wrap(s string, width int, indent string) string {
	words := strings.Fields(s)
	var lines []string
	line := ""
	for _, w := range words {
		if len(line)+len(w)+1 > width && line != "" {
			lines = append(lines, line)
			line = indent + w
		} else if line == "" {
			line = w
		} else {
			line += " " + w
		}
	}
	if line != "" {
		lines = append(lines, line)
	}
	return strings.Join(lines, "\n")
}