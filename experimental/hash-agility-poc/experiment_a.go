// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"strings"
)

// RunExperimentA attempts to patch gittuf's RSL verification by injecting an
// in-memory SHA-1->SHA-256 hash translation layer.
//
// The key insight we test here is: even if we can translate the TargetID hash
// stored in an RSL entry's commit message, can we still verify the entry?
// Answer: NO -- because the commit message is part of the signed data.
// Changing it would invalidate the signature.
//
// We demonstrate this in 3 sub-steps:
//  1. Parse an RSL entry from the SHA-1 repo -- get the raw commit message
//  2. Show that the TargetID in the message is SHA-1 and fails in SHA-256 repo
//  3. Attempt translation and show the signature verification path would break
func RunExperimentA(migration *MigrationResult, sha1State *SHA1RepoState) *ExperimentResult {
	result := &ExperimentResult{}

	// Step A1: Read RSL entry commit message from SHA-1 repo
	fmt.Println("  [A1] Reading RSL entry commit messages from SHA-1 repo")
	if len(sha1State.RSLEntrySHA1s) == 0 {
		result.Findings = append(result.Findings, Finding{
			Description: "No RSL entries found in SHA-1 repo -- cannot proceed",
			Passed:      false,
		})
		result.Verdict = "INCONCLUSIVE: No RSL entries to analyze"
		return result
	}

	lastRSLCommit := sha1State.RSLEntrySHA1s[len(sha1State.RSLEntrySHA1s)-1]
	commitMsg, err := runGit(sha1State.RepoPath, "log", "--format=%B", "-1", lastRSLCommit)
	if err != nil {
		result.Findings = append(result.Findings, Finding{
			Description: fmt.Sprintf("Failed to read RSL commit message: %v", err),
			Passed:      false,
		})
		result.Verdict = "INCONCLUSIVE: Could not read RSL entries"
		return result
	}

	fmt.Printf("  [A1] RSL commit message:\n%s\n", indentLines(commitMsg, "       "))
	result.Findings = append(result.Findings, Finding{
		Description: fmt.Sprintf("RSL entry commit message parsed (SHA-1: %s...)", lastRSLCommit[:16]),
		Passed:      true,
	})

	// Step A2: Extract the targetID from the commit message
	targetID := extractTargetID(commitMsg)
	fmt.Printf("  [A2] Extracted targetID from RSL entry: %q\n", targetID)
	if targetID == "" {
		result.Findings = append(result.Findings, Finding{
			Description: "Could not extract targetID from RSL commit message",
			Passed:      false,
		})
	} else {
		result.Findings = append(result.Findings, Finding{
			Description: fmt.Sprintf("targetID in RSL entry is SHA-1: %s", targetID[:16]+"..."),
			Passed:      true,
		})
	}

	// Step A3: Check if targetID exists in SHA-256 repo
	fmt.Println("  [A3] Checking if SHA-1 targetID resolves in the SHA-256 repo")
	_, lookupErr := runGit(migration.SHA256RepoPath, "cat-file", "-t", targetID)
	if lookupErr != nil {
		result.Findings = append(result.Findings, Finding{
			Description: fmt.Sprintf(
				"SHA-1 targetID '%s...' does NOT resolve in SHA-256 repo: %v",
				targetID[:16], lookupErr),
			Passed: false,
		})
	} else {
		result.Findings = append(result.Findings, Finding{
			Description: fmt.Sprintf("SHA-1 targetID resolves in SHA-256 repo (compat mapping works)"),
			Passed:      true,
		})
	}

	// Step A4: Attempt translation using our map
	fmt.Println("  [A4] Attempting SHA-1 -> SHA-256 translation")
	translatedID, ok := migration.SHA1ToSHA256[targetID]
	if ok {
		fmt.Printf("  [A4] Translated: %s -> %s\n", targetID[:16]+"...", translatedID[:16]+"...")
		result.Findings = append(result.Findings, Finding{
			Description: fmt.Sprintf("Hash translation SUCCEEDED: SHA-1 %s -> SHA-256 %s",
				targetID[:16]+"...", translatedID[:16]+"..."),
			Passed: true,
		})
	} else {
		result.Findings = append(result.Findings, Finding{
			Description: fmt.Sprintf("Hash translation FAILED: SHA-1 %s not in translation map", targetID[:16]+"..."),
			Passed:      false,
		})
	}

	// Step A5: THE KEY FINDING -- Signature verification
	// Even if we translate the hash in memory, the RSL *commit message* still
	// contains the SHA-1 hash. The git commit signature (SSH/GPG) signs the
	// entire commit object, which includes the message. We cannot modify the
	// message without invalidating the signature.
	fmt.Println("  [A5] Checking RSL entry commit signature (the fundamental blocker)")
	sigCheck, sigErr := runGit(sha1State.RepoPath, "log", "--format=%GS", "-1", lastRSLCommit)
	if sigErr == nil && strings.TrimSpace(sigCheck) != "" {
		result.Findings = append(result.Findings, Finding{
			Description: fmt.Sprintf(
				"RSL entry IS signed (%s). To change the SHA-1 in the commit message, "+
					"we would need to rewrite the commit -> SIGNATURE BREAKS. "+
					"Translation layer CANNOT fix signed entries.",
				strings.TrimSpace(sigCheck)),
			Passed: false,
		})
	} else {
		// Entry is not signed (unsigned test mode) -- note this for context
		result.Findings = append(result.Findings, Finding{
			Description: "RSL entry is unsigned (test/dev mode). In production, entries " +
				"ARE signed. Rewriting the commit message to replace SHA-1 hashes " +
				"WOULD break signatures. Translation layer is fundamentally unsound.",
			Passed: false,
		})
	}

	result.Verdict = "APPROACH A REJECTED: In-memory hash translation is partially possible " +
		"for object lookups, but fundamentally fails for signature verification. " +
		"Any modification of signed RSL commit messages to replace SHA-1 hashes " +
		"invalidates the cryptographic signatures, defeating gittuf's security model."

	return result
}

// extractTargetID parses a targetID from an RSL commit message.
// Format: "targetID: <hash>"
func extractTargetID(commitMsg string) string {
	for _, line := range strings.Split(commitMsg, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "targetID:") {
			parts := strings.SplitN(line, ":", 2)
			if len(parts) == 2 {
				return strings.TrimSpace(parts[1])
			}
		}
	}
	return ""
}

// indentLines adds a prefix to every line in a string.
func indentLines(s, prefix string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	for i, l := range lines {
		lines[i] = prefix + l
	}
	return strings.Join(lines, "\n")
}
