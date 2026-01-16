// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/jonboulle/clockwork"
)

const (
	testName  = "Jane Doe"
	testEmail = "jane.doe@example.com"
)

var (
	testClock = clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
)

// CreateTestGitRepository creates a Git repository in the specified directory.
// This is meant to be used by tests across gittuf packages. This helper also
// sets up an ED25519 signing key that can be used to create reproducible
// commits.
func CreateTestGitRepository(t *testing.T, dir string, bare bool) *Repository {
	t.Helper()

	repo := setupRepository(t, dir, bare)

	// Set up author / committer identity
	if err := repo.SetGitConfig("user.name", testName); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetGitConfig("user.email", testEmail); err != nil {
		t.Fatal(err)
	}

	// Set up signing via SSH key
	keysDir := t.TempDir()
	setupSigningKeys(t, keysDir)

	if err := repo.SetGitConfig("user.signingkey", filepath.Join(keysDir, "key.pub")); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetGitConfig("gpg.format", "ssh"); err != nil {
		t.Fatal(err)
	}

	return repo
}

func setupRepository(t *testing.T, dir string, bare bool) *Repository {
	t.Helper()

	var gitDirPath string
	args := []string{"init"}
	if bare {
		args = append(args, "--bare")
		gitDirPath = dir
	} else {
		gitDirPath = filepath.Join(dir, ".git")
	}
	args = append(args, "-b", "main")
	args = append(args, dir)

	cmd := exec.Command(binary, args...)
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	return &Repository{gitDirPath: gitDirPath, clock: testClock}
}

func setupSigningKeys(t *testing.T, dir string) {
	t.Helper()

	sshPrivateKey := artifacts.SSHRSAPrivate
	sshPublicKey := artifacts.SSHRSAPublicSSH

	privateKeyPath := filepath.Join(dir, "key")
	publicKeyPath := filepath.Join(dir, "key.pub")

	if err := os.WriteFile(privateKeyPath, sshPrivateKey, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(publicKeyPath, sshPublicKey, 0o600); err != nil {
		t.Fatal(err)
	}
}
