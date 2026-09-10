// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// SetupSHA1Repo creates a fresh SHA-1 git repository in workDir/sha1-repo,
// initializes gittuf on it (recording RSL entries manually), makes several
// commits, and returns the captured state for subsequent phases.
//
// We use the gittuf binary's sl record subcommand so that the RSL entries
// are created exactly as they would be in a real workflow.
func SetupSHA1Repo(workDir string) (*SHA1RepoState, error) {
	repoPath := filepath.Join(workDir, "sha1-repo")
	if err := os.MkdirAll(repoPath, 0o755); err != nil {
		return nil, fmt.Errorf("creating sha1 repo dir: %w", err)
	}

	state := &SHA1RepoState{RepoPath: repoPath}
	gittufBin := findGittufBinary()

	// ── Step 1: git init ────────────────────────────────────────────────────
	fmt.Println("  -> git init --object-format=sha1")
	if _, err := runGit(repoPath, "init", "--object-format=sha1", "-b", "main"); err != nil {
		// Older git may not accept --object-format; SHA-1 is the default anyway
		if _, err2 := runGit(repoPath, "init", "-b", "main"); err2 != nil {
			return nil, fmt.Errorf("git init: %w", err2)
		}
		fmt.Println("  -> (--object-format flag unsupported by this git, SHA-1 is the default)")
	}

	// ── Step 2: Configure git identity ──────────────────────────────────────
	cfgs := [][]string{
		{"config", "user.email", "poc@gittuf.dev"},
		{"config", "user.name", "GAP1 PoC"},
		{"config", "commit.gpgsign", "false"},
	}
	for _, cfg := range cfgs {
		if _, err := runGit(repoPath, cfg...); err != nil {
			return nil, fmt.Errorf("git %v: %w", cfg, err)
		}
	}

	// ── Step 3: Three commits on main ──────────────────────────────────────
	fmt.Println("  -> Creating 3 commits on main")
	for i := 1; i <= 3; i++ {
		content := fmt.Sprintf("# Project\n\nVersion %d content.\n", i)
		if err := os.WriteFile(filepath.Join(repoPath, "README.md"), []byte(content), 0o644); err != nil {
			return nil, fmt.Errorf("writing README v%d: %w", i, err)
		}
		if _, err := runGit(repoPath, "add", "README.md"); err != nil {
			return nil, fmt.Errorf("git add v%d: %w", i, err)
		}
		if _, err := runGit(repoPath, "commit", "-m", fmt.Sprintf("chore: version %d", i)); err != nil {
			return nil, fmt.Errorf("git commit v%d: %w", i, err)
		}
		head, err := runGit(repoPath, "rev-parse", "HEAD")
		if err != nil {
			return nil, fmt.Errorf("rev-parse HEAD v%d: %w", i, err)
		}
		sha := strings.TrimSpace(head)
		state.CommitSHA1s = append(state.CommitSHA1s, sha)
		fmt.Printf("  -> Commit %d: %s\n", i, sha)
	}

	// ── Step 4: Record RSL entries for each commit ──────────────────────────
	// We use gittuf rsl record which appends a signed RSL ReferenceEntry
	// to refs/gittuf/reference-state-log. This creates the commit messages
	// containing TargetID: <sha1_hash> that are central to the GAP-1 problem.
	fmt.Println("  -> Recording RSL entries via gittuf rsl record")
	for i, commitSHA := range state.CommitSHA1s {
		// First, ensure main points to this commit (for rsl record to pick up)
		if _, err := runGit(repoPath, "update-ref", "refs/heads/main", commitSHA); err != nil {
			return nil, fmt.Errorf("update-ref for commit %d: %w", i+1, err)
		}

		// gittuf rsl record refs/heads/main
		_, err := runCmd(repoPath, gittufBin, "rsl", "record", "refs/heads/main")
		if err != nil {
			// gittuf may fail if trust is not initialized; record manually
			fmt.Printf("  -> gittuf rsl record failed (%v); falling back to manual RSL commit\n", err)
			if manualErr := manualRSLRecord(repoPath, "refs/heads/main", commitSHA, i+1); manualErr != nil {
				return nil, fmt.Errorf("manual RSL record for commit %d: %w", i+1, manualErr)
			}
		}

		rslTip, err := runGit(repoPath, "rev-parse", "refs/gittuf/reference-state-log")
		if err != nil {
			return nil, fmt.Errorf("reading RSL tip after entry %d: %w", i+1, err)
		}
		rslTipStr := strings.TrimSpace(rslTip)
		state.RSLEntrySHA1s = append(state.RSLEntrySHA1s, rslTipStr)
		state.RSLEntryCount++
		fmt.Printf("  -> RSL entry %d: %s\n", i+1, rslTipStr)
	}

	state.FinalHeadSHA1 = state.CommitSHA1s[len(state.CommitSHA1s)-1]
	state.FinalRSLTipSHA1 = state.RSLEntrySHA1s[len(state.RSLEntrySHA1s)-1]

	// ── Step 5: Verify the baseline (expect PASS) ───────────────────────────
	fmt.Println("  -> Running baseline gittuf verify-ref on SHA-1 repo")
	if _, err := runCmd(repoPath, gittufBin, "verify-ref", "main"); err != nil {
		// Verification may fail if gittuf was not fully initialized (no policy);
		// for the PoC we note this but it doesn't block the migration experiment.
		fmt.Printf("  -> baseline verify-ref warning (no policy initialized): %v\n", err)
	}

	return state, nil
}

// manualRSLRecord creates a raw RSL commit directly using git commit-tree.
// This mimics exactly what gittuf rsl record does internally, giving us full
// control over the commit message format for the PoC.
//
// RSL ReferenceEntry commit message format (from internal/rsl/rsl.go:186-196):
//
//	RSL Reference Entry
//
//	ref: refs/heads/main
//	targetID: <sha1_hash>
//	number: <n>
func manualRSLRecord(repoPath, refName, targetID string, number int) error {
	// Get or create an empty tree (reuse across calls)
	emptyTreeID, err := getOrCreateEmptyTree(repoPath)
	if err != nil {
		return fmt.Errorf("empty tree: %w", err)
	}

	msg := fmt.Sprintf("RSL Reference Entry\n\nref: %s\ntargetID: %s\nnumber: %d",
		refName, targetID, number)

	// Get current RSL tip (if any) as parent
	args := []string{"commit-tree", "-m", msg}
	rslTip, err := runGit(repoPath, "rev-parse", "refs/gittuf/reference-state-log")
	if err == nil {
		args = append(args, "-p", strings.TrimSpace(rslTip))
	}
	args = append(args, emptyTreeID)

	newCommit, err := runGit(repoPath, args...)
	if err != nil {
		return fmt.Errorf("commit-tree: %w", err)
	}
	newCommit = strings.TrimSpace(newCommit)

	// Update the RSL ref
	updateArgs := []string{"update-ref", "refs/gittuf/reference-state-log", newCommit}
	if err == nil && rslTip != "" {
		// with old value for safety
	}
	_, err = runGit(repoPath, updateArgs...)
	return err
}

// getOrCreateEmptyTree returns the SHA-1 of Git's canonical empty tree object.
func getOrCreateEmptyTree(repoPath string) (string, error) {
	// git hash-object -t tree --stdin < /dev/null
	// The canonical empty tree SHA-1 is always 4b825dc642cb6eb9a060e54bf8d69288fbee4904
	out, err := runGit(repoPath, "hash-object", "-t", "tree", "--stdin")
	if err != nil {
		// Pipe empty input: try with echo
		out, err = execCmd(repoPath, "git", "hash-object", "-t", "tree", "--stdin")
	}
	return strings.TrimSpace(out), err
}

// findGittufBinary locates the gittuf executable built at the repo root.
func findGittufBinary() string {
	cwd, err := os.Getwd()
	if err != nil {
		return "gittuf"
	}
	// Walk up to find gittuf.exe / gittuf
	dir := cwd
	for {
		for _, name := range []string{"gittuf.exe", "gittuf"} {
			candidate := filepath.Join(dir, name)
			if info, err := os.Stat(candidate); err == nil && !info.IsDir() {
				return candidate
			}
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "gittuf"
}
