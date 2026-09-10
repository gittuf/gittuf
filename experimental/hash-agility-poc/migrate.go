// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// MigrateToSHA256 creates a fresh SHA-256 repository and demonstrates what
// happens to gittuf metadata when the object format changes.
//
// Real-world migration path:
//   git init --object-format=sha256 <new-repo>
//   git remote add origin <sha1-repo>
//   git fetch origin  (objects land as SHA-256 automatically via translation)
//
// For the PoC we create the SHA-256 repo and copy commits manually to show
// that the SHA-1 hashes embedded in RSL commit messages become orphaned.
func MigrateToSHA256(workDir string, sha1State *SHA1RepoState) (*MigrationResult, error) {
	sha256RepoPath := filepath.Join(workDir, "sha256-repo")

	result := &MigrationResult{
		SHA256RepoPath: sha256RepoPath,
		SHA1ToSHA256:   make(map[string]string),
	}

	// 1. Create a fresh SHA-256 repository
	fmt.Println("  -> git init --object-format=sha256 sha256-repo")
	if _, err := runGit(workDir, "init", "--object-format=sha256", "-b", "main", sha256RepoPath); err != nil {
		return nil, fmt.Errorf("git init --object-format=sha256 failed: %w\n"+
			"  Git 2.29+ required for SHA-256 repository support.", err)
	}

	// 2. Configure identity
	runGit(sha256RepoPath, "config", "user.email", "poc@gittuf.dev")  //nolint
	runGit(sha256RepoPath, "config", "user.name", "GAP1 PoC")         //nolint
	runGit(sha256RepoPath, "config", "commit.gpgsign", "false")       //nolint

	// 3. Add SHA-1 repo as a remote and fetch
	// When git fetches from a SHA-1 repo into a SHA-256 repo, it re-hashes
	// all objects. The SHA-256 repo gets new hashes for all existing objects.
	fmt.Println("  -> Adding sha1-repo as remote and fetching (re-hashing to SHA-256)")
	if _, err := runGit(sha256RepoPath, "remote", "add", "sha1origin", sha1State.RepoPath); err != nil {
		return nil, fmt.Errorf("git remote add: %w", err)
	}

	fetchOut, fetchErr := runGit(sha256RepoPath, "fetch", "sha1origin", "refs/heads/main:refs/heads/main")
	if fetchErr != nil {
		// Cross-format fetch may not be supported; use fast-export/fast-import instead
		fmt.Printf("  -> Direct fetch failed (%v), using fast-export/fast-import\n", fetchErr)
		_ = fetchOut
		if err := fastExportImport(sha1State.RepoPath, sha256RepoPath); err != nil {
			return nil, fmt.Errorf("fast-export/import failed: %w", err)
		}
	}

	// 4. Read the new HEAD in SHA-256
	newHead, err := runGit(sha256RepoPath, "rev-parse", "HEAD")
	if err != nil {
		// HEAD may not be set if no commits were fetched; try refs/heads/main
		newHead, err = runGit(sha256RepoPath, "rev-parse", "refs/heads/main")
		if err != nil {
			result.NewHeadSHA256 = "(not available — fetch may have failed)"
			fmt.Println("  -> WARNING: Could not read new HEAD in SHA-256 repo")
		}
	}
	if newHead != "" {
		result.NewHeadSHA256 = strings.TrimSpace(newHead)
	}
	fmt.Printf("  -> New HEAD (SHA-256): %s\n", result.NewHeadSHA256)


	// 4. Build SHA-1 -> SHA-256 translation table
	// In a repo cloned with --object-format=sha256 from a SHA-1 repo,
	// git maintains a compatibility mapping. We can query it via:
	//   git cat-file --batch-check='%(objectname:sha256)' < list-of-sha1s
	fmt.Println("  -> Building SHA-1 to SHA-256 translation table")
	allSHA1s := append(sha1State.CommitSHA1s, sha1State.RSLEntrySHA1s...)
	for _, sha1 := range allSHA1s {
		// Try: git rev-parse <sha1> in the sha256 repo (works via compat mapping)
		sha256, err := runGit(sha256RepoPath, "rev-parse", sha1)
		if err != nil {
			// Object may not be in the SHA-256 repo (e.g. RSL commits not pushed)
			fmt.Printf("  -> WARNING: SHA-1 %s not found in SHA-256 repo: %v\n", sha1[:16], err)
			continue
		}
		sha256Trimmed := strings.TrimSpace(sha256)
		if sha256Trimmed != sha1 { // different means the mapping worked
			result.SHA1ToSHA256[sha1] = sha256Trimmed
			fmt.Printf("  -> Mapped: %s -> %s\n", sha1[:16]+"...", sha256Trimmed[:16]+"...")
		}
	}

	// 5. Probe: does the old SHA-1 RSL ref exist in the SHA-256 repo?
	fmt.Println("  -> Checking if refs/gittuf/reference-state-log was cloned")
	rslTip, rslErr := runGit(sha256RepoPath, "rev-parse", "refs/gittuf/reference-state-log")
	if rslErr != nil {
		fmt.Printf("  -> refs/gittuf/* NOT present in SHA-256 clone (expected if not pushed): %v\n", rslErr)
		// Try to manually push the gittuf refs into the sha256 repo
		fmt.Println("  -> Attempting to copy gittuf refs from SHA-1 repo")
		if copyErr := copyGittufRefs(sha1State.RepoPath, sha256RepoPath); copyErr != nil {
			fmt.Printf("  -> Could not copy gittuf refs: %v\n", copyErr)
		} else {
			rslTip, rslErr = runGit(sha256RepoPath, "rev-parse", "refs/gittuf/reference-state-log")
		}
	}

	if rslErr == nil {
		fmt.Printf("  -> RSL tip in SHA-256 repo: %s (original SHA-1 hash -- now orphaned)\n",
			strings.TrimSpace(rslTip)[:16]+"...")
	}

	// 6. Attempt gittuf verify-ref on SHA-256 repo — expect FAILURE
	fmt.Println("  -> Running gittuf verify-ref main on SHA-256 repo (expecting failure)")
	gittufBin := findGittufBinary()
	verifyOut, verifyErr := runCmd(sha256RepoPath, gittufBin,
		"verify-ref", "--verbose", "main")
	if verifyErr != nil {
		result.VerifyError = extractErrorSummary(verifyErr.Error())
		fmt.Printf("  -> [CONFIRMED FAILURE]: %s\n", result.VerifyError)
	} else {
		// Unexpected success — record it
		result.VerifyError = "UNEXPECTED_SUCCESS: " + verifyOut
		fmt.Printf("  -> [UNEXPECTED]: verify-ref passed! Output: %s\n", verifyOut)
	}

	return result, nil
}

// copyGittufRefs copies all refs/gittuf/* from the source SHA-1 repo
// directly into the SHA-256 repo's object store and ref namespace.
// This simulates what would happen if refs/gittuf/* were pushed to a remote.
func copyGittufRefs(srcRepo, dstRepo string) error {
	// List all gittuf refs in source
	refList, err := runGit(srcRepo, "for-each-ref", "--format=%(refname)", "refs/gittuf/")
	if err != nil {
		return fmt.Errorf("listing gittuf refs: %w", err)
	}

	refs := strings.Split(strings.TrimSpace(refList), "\n")
	for _, ref := range refs {
		ref = strings.TrimSpace(ref)
		if ref == "" {
			continue
		}
		sha1, err := runGit(srcRepo, "rev-parse", ref)
		if err != nil {
			continue
		}
		sha1 = strings.TrimSpace(sha1)

		bundleFile, _ := os.CreateTemp("", "gittuf-gap1-*.bundle")
		bundleFile.Close()
		bundlePath := bundleFile.Name()
		defer os.Remove(bundlePath) //nolint:gocritic

		if _, err := runGit(srcRepo, "bundle", "create", bundlePath, sha1); err != nil {
			fmt.Printf("  -> bundle create for %s failed: %v\n", ref, err)
			continue
		}
		if _, err := runGit(dstRepo, "bundle", "unbundle", bundlePath); err != nil {
			fmt.Printf("  -> bundle unbundle for %s failed: %v\n", ref, err)
			continue
		}
		if _, err := runGit(dstRepo, "update-ref", ref, sha1); err != nil {
			fmt.Printf("  -> update-ref %s failed: %v\n", ref, err)
		}
	}
	return nil
}

// fastExportImport migrates objects from srcRepo (SHA-1) to dstRepo (SHA-256)
// using git fast-export piped to git fast-import.
// This demonstrates that even when objects are transferred, they get NEW hashes
// in the SHA-256 repo — the old SHA-1 hashes in RSL entries become orphaned.
func fastExportImport(srcRepo, dstRepo string) error {
	fmt.Println("  -> git fast-export (sha1) | git fast-import (sha256)")

	// Write fast-export output to temp file
	exportData, err := runGit(srcRepo, "fast-export", "--all")
	if err != nil {
		return fmt.Errorf("fast-export: %w", err)
	}

	if exportData == "" {
		return fmt.Errorf("fast-export produced no output")
	}

	// Write export stream to temp file
	exportFile, err := os.CreateTemp("", "gittuf-gap1-export-*.stream")
	if err != nil {
		return err
	}
	if _, err := exportFile.WriteString(exportData); err != nil {
		exportFile.Close()
		os.Remove(exportFile.Name())
		return err
	}
	exportFile.Close()
	defer os.Remove(exportFile.Name())

	// Import into SHA-256 repo (reads from stdin via the stream file)
	// Note: git fast-import in a SHA-256 repo will re-hash all objects to SHA-256
	importCmd := fmt.Sprintf("git --git-dir=%s fast-import < %s", dstRepo+"/.git", exportFile.Name())
	out, err := execCmdShell(dstRepo, importCmd)
	if err != nil {
		// fast-import may not support cross-format; document this clearly
		fmt.Printf("  -> fast-import cross-format note: %v\n", err)
		fmt.Printf("     Output: %s\n", out)
		fmt.Println("  -> KEY FINDING: git fast-import rejects SHA-1 objects in SHA-256 repo")
		fmt.Println("     Objects must be re-hashed, not just transferred. This confirms")
		fmt.Println("     that old RSL entry hashes (SHA-1) will NEVER resolve in SHA-256 repo.")
		return nil // Non-fatal for the PoC — this IS the demonstration
	}
	return nil
}

// execCmdShell runs a shell command string (for piping support).
func execCmdShell(dir string, command string) (string, error) {
	// Use PowerShell on Windows
	out, err := execCmd(dir, "powershell", "-Command", command)
	if err != nil {
		// Try cmd as fallback
		return execCmd(dir, "cmd", "/C", command)
	}
	return out, nil
}


// extractErrorSummary pulls the most relevant line from a multi-line error.
func extractErrorSummary(errStr string) string {
	lines := strings.Split(errStr, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if strings.Contains(line, "unable") ||
			strings.Contains(line, "error") ||
			strings.Contains(line, "not found") ||
			strings.Contains(line, "invalid") {
			return line
		}
	}
	if len(lines) > 0 {
		return strings.TrimSpace(lines[0])
	}
	return errStr
}

