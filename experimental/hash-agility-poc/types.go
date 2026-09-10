// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

// SHA1RepoState holds all state captured from the baseline SHA-1 repository.
type SHA1RepoState struct {
	RepoPath        string
	FinalHeadSHA1   string
	FinalRSLTipSHA1 string
	RSLEntryCount   int
	CommitSHA1s     []string
	RSLEntrySHA1s   []string
}

// MigrationResult holds the results of the SHA-256 migration step.
type MigrationResult struct {
	SHA256RepoPath string
	NewHeadSHA256  string
	VerifyError    string
	SHA1ToSHA256   map[string]string
}

// ExperimentResult holds the outcome of a single experiment (A or B).
type ExperimentResult struct {
	Findings         []Finding
	Verdict          string
	SnapshotManifest *SnapshotManifest
}

// Finding is a single pass/fail observation within an experiment.
type Finding struct {
	Description string
	Passed      bool
}

// SnapshotManifest is the cryptographic record of the frozen SHA-1 repo state.
type SnapshotManifest struct {
	SchemaVersion string    `json:"schema_version"`
	FrozenAt      time.Time `json:"frozen_at"`
	SHA1RepoHead  string    `json:"sha1_repo_head"`
	RSLTip        string    `json:"rsl_tip"`
	RSLEntryCount int       `json:"rsl_entry_count"`
	RSLChainHash  string    `json:"rsl_chain_merkle_hash"`
	MigrationNote string    `json:"migration_note"`
	SignedBy      string    `json:"signed_by"`
}

// SaveSnapshotManifest writes the snapshot manifest to workDir as JSON.
func SaveSnapshotManifest(workDir string, m *SnapshotManifest) (string, error) {
	data, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return "", err
	}
	path := filepath.Join(workDir, "snapshot-manifest.json")
	return path, os.WriteFile(path, data, 0o644)
}

// runGit runs a git command in the given directory, returning trimmed stdout.
func runGit(dir string, args ...string) (string, error) {
	return execCmd(dir, "git", args...)
}