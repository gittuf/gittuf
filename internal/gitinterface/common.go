// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"testing"
	"time"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/go-git/go-git/v5/config"
	"github.com/jonboulle/clockwork"
)

const (
	testName  = "Jane Doe"
	testEmail = "jane.doe@example.com"
)

var (
	testGitConfig = &config.Config{
		User: struct {
			Name  string
			Email string
		}{
			Name:  testName,
			Email: testEmail,
		},
	}
	testClock = clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
)

// CreateTestGitRepository creates a Git repository in the specified directory.
// This is meant to be used by tests across gittuf packages. This helper also
// sets up an ED25519 signing key that can be used to create reproducible
// commits.
func CreateTestGitRepository(t *testing.T, dir string) *Repository {
	t.Helper()

	keysDir := t.TempDir()

	setupSigningKeys(t, keysDir)

	if err := os.Chdir(dir); err != nil {
		t.Fatal(err)
	}

	cmd := exec.Command(binary, "init", "-b", "main")
	if err := cmd.Run(); err != nil {
		t.Fatal(err)
	}

	repo := &Repository{gitDirPath: path.Join(dir, ".git"), clock: testClock}

	// Set up author / committer identity
	if err := repo.SetGitConfig("user.name", testName); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetGitConfig("user.email", testEmail); err != nil {
		t.Fatal(err)
	}

	// Set up signing via ED25519 SSH key (deterministic sigs!)
	if err := repo.SetGitConfig("user.signingkey", filepath.Join(keysDir, "key")); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetGitConfig("gpg.format", "ssh"); err != nil {
		t.Fatal(err)
	}

	return repo
}

func setupSigningKeys(t *testing.T, dir string) {
	t.Helper()

	sshPrivateKey := artifacts.SSHED25519Private
	sshPublicKey := artifacts.SSHED25519PublicSSH

	privateKeyPath := filepath.Join(dir, "key")
	publicKeyPath := filepath.Join(dir, "key.pub")

	if err := os.WriteFile(privateKeyPath, sshPrivateKey, 0o600); err != nil {
		t.Fatal(err)
	}

	if err := os.WriteFile(publicKeyPath, sshPublicKey, 0o600); err != nil {
		t.Fatal(err)
	}
}
