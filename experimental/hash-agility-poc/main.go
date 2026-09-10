// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Command hash-agility-poc is the entry point for the GAP-1 Hash Agility
// Proof of Concept. It demonstrates the SHA-1 -> SHA-256 migration failure
// in gittuf and experimentally evaluates three approaches:
//
//   Approach A: In-memory SHA-1->SHA-256 hash translation layer
//   Approach B: Cryptographic snapshot + fresh gittuf initialization
//   Approach C: Cross-signing in-toto attestations (hash equivalence)
//
// Run with: go run ./experimental/hash-agility-poc/
package main

import (
	"fmt"
	"os"
)

func main() {
	fmt.Println("=======================================================")
	fmt.Println("  GAP-1 Hash Agility PoC -- gittuf SHA-1 to SHA-256  ")
	fmt.Println("=======================================================")
	fmt.Println()

	workDir, err := os.MkdirTemp("", "gittuf-gap1-poc-*")
	if err != nil {
		fatalf("Failed to create working directory: %v", err)
	}
	defer os.RemoveAll(workDir)

	fmt.Printf("Working directory: %s\n\n", workDir)

	// Phase 1: Setup SHA-1 repo with gittuf
	printPhaseHeader(1, "Setup SHA-1 Repository with Gittuf")
	sha1State, err := SetupSHA1Repo(workDir)
	if err != nil {
		fatalf("Phase 1 failed: %v", err)
	}
	printSuccess("SHA-1 repo created and gittuf initialized")
	printSuccess(fmt.Sprintf("RSL has %d entries", sha1State.RSLEntryCount))
	printSuccess(fmt.Sprintf("Final HEAD (SHA-1): %s", sha1State.FinalHeadSHA1))
	printSuccess(fmt.Sprintf("Final RSL tip (SHA-1): %s", sha1State.FinalRSLTipSHA1))
	printSuccess("gittuf verify-ref PASSED on SHA-1 repo")
	fmt.Println()

	// Phase 2: Migrate to SHA-256
	printPhaseHeader(2, "Migrate to SHA-256 Repository")
	migrateResult, err := MigrateToSHA256(workDir, sha1State)
	if err != nil {
		fatalf("Phase 2 failed: %v", err)
	}
	printSuccess(fmt.Sprintf("SHA-256 repo created at: %s", migrateResult.SHA256RepoPath))
	printSuccess(fmt.Sprintf("New HEAD (SHA-256): %s", migrateResult.NewHeadSHA256))
	printWarning(fmt.Sprintf("gittuf verify-ref FAILED as expected: %s", migrateResult.VerifyError))
	fmt.Println()

	// Phase 3A: Translation layer experiment
	printPhaseHeader(3, "Experiment A -- In-Memory Hash Translation Layer")
	resultA := RunExperimentA(migrateResult, sha1State)
	printExperimentResult("A", resultA)
	fmt.Println()

	// Phase 3B: Snapshot + fresh start experiment
	printPhaseHeader(4, "Experiment B -- Cryptographic Snapshot + Fresh Start")
	resultB := RunExperimentB(workDir, sha1State, migrateResult)
	printExperimentResult("B", resultB)
	fmt.Println()

	// Phase 3C: Cross-signing attestations experiment
	printPhaseHeader(5, "Experiment C -- Cross-Signing in-toto Attestation")
	resultC := RunExperimentC(workDir, sha1State, migrateResult)
	printExperimentResult("C", resultC)
	fmt.Println()

	// Final report
	PrintFinalReport(sha1State, migrateResult, resultA, resultB, resultC)
}

func printPhaseHeader(n int, title string) {
	fmt.Printf("-------------------------------------------------------\n")
	fmt.Printf("  Phase %d: %s\n", n, title)
	fmt.Printf("-------------------------------------------------------\n")
}

func printSuccess(msg string) { fmt.Printf("  [PASS] %s\n", msg) }
func printWarning(msg string) { fmt.Printf("  [WARN] %s\n", msg) }
func printError(msg string)   { fmt.Printf("  [FAIL] %s\n", msg) }
func printInfo(msg string)    { fmt.Printf("  [INFO] %s\n", msg) }

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "FATAL: "+format+"\n", args...)
	os.Exit(1)
}

func printExperimentResult(label string, r *ExperimentResult) {
	fmt.Printf("  Approach %s result:\n", label)
	for _, f := range r.Findings {
		if f.Passed {
			printSuccess(f.Description)
		} else {
			printError(f.Description)
		}
	}
	if r.Verdict != "" {
		printInfo(fmt.Sprintf("Verdict: %s", r.Verdict))
	}
}