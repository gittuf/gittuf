// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gitinterface

import (
	"bytes"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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

func CreateTestGitRepositoryForGPGSigning(t *testing.T, dir string, bare bool) *Repository {
	t.Helper()

	repo := setupRepository(t, dir, bare)

	// Set up author / committer identity
	if err := repo.SetGitConfig("user.name", testName); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetGitConfig("user.email", testEmail); err != nil {
		t.Fatal(err)
	}

	// Set up signing via GPG key
	keysDir := t.TempDir()
	os.Setenv("GNUPGHOME", keysDir)
	defer os.Unsetenv("GNUPGHOME")

	err := os.Chmod(keysDir, 0o700)
	if err != nil {
		t.Fatalf("chmod GNUPGHOME failed: %v", err)
	}

	if err := os.Chmod(keysDir, 0o700); err != nil {
		t.Fatalf("chmod GNUPGHOME failed: %v", err)
	}
	t.Logf("Set GNUPGHOME to: %s", keysDir)

	// Configure gpg-agent?
	agentConfPath := filepath.Join(keysDir, "gpg-agent.conf")
	agentConfContent := `
allow-loopback-tty
pinentry-program /bin/true
`
	if err := os.WriteFile(agentConfPath, []byte(strings.TrimSpace(agentConfContent)), 0600); err != nil {
		t.Fatalf("failed to write gpg-agent.conf to '%s': %v", agentConfPath, err)
	}

	fp := setupGPGSigningKeys(t, keysDir)

	if err := repo.SetGitConfig("user.signingkey", fp); err != nil {
		t.Fatal(err)
	}
	if err := repo.SetGitConfig("gpg.format", "gpg"); err != nil {
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

// func setupGPGSigningKeys(t *testing.T, dir string) {
// 	t.Helper()

// 	gpgPrivateKey := artifacts.GPGKey1Private
// 	gpgPublicKey := artifacts.GPGKey1Public
// 	gpgPrivateKeyPath := filepath.Join(dir, "gpgKey")
// 	gpgPublicKeyPath := filepath.Join(dir, "gpgKey.pub")

// 	if err := os.WriteFile(gpgPrivateKeyPath, gpgPrivateKey, 0o600); err != nil {
// 		t.Fatal(err)
// 	}

// 	if err := os.WriteFile(gpgPublicKeyPath, gpgPublicKey, 0o600); err != nil {
// 		t.Fatal(err)
// 	}

// 	cmd := exec.Command("gpg", "--batch", "--yes", "--pinentry-mode", "loopback", "--passphrase", "test", "--import", gpgPrivateKeyPath)
// 	cmd.Env = append(os.Environ(), "GNUPGHOME="+dir)
// 	output, err := cmd.CombinedOutput()
// 	if err != nil {
// 		t.Fatalf("gpg --import %s failed: %v \n%s", gpgPrivateKeyPath, err, output)
// 	}

// 	cmd = exec.Command("gpg", "--batch", "--yes", "--pinentry-mode", "loopback", "--import", gpgPublicKeyPath)
// 	cmd.Env = append(os.Environ(), "GNUPGHOME="+dir)
// 	output, err = cmd.CombinedOutput()
// 	if err != nil {
// 		t.Fatalf("gpg --import %s failed: %v \n%s", gpgPublicKeyPath, err, output)
// 	}
// }

func setupGPGSigningKeys(t *testing.T, dir string) string {
	t.Helper()

	// Generate key for temporary test environment
	cmd := exec.Command(
		"gpg", "--batch", "--homedir", dir,
		"--pinentry-mode", "loopback", "--passphrase", "",
		"--quick-gen-key", "Test User <test@example.com>", "rsa2048", "sign", "0",
	)
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("gpg --quick-gen-key failed: %v\n%s", err, output)
	}

	// Extract fingerprint from the generated key
	cmd = exec.Command(
		"gpg", "--batch", "--homedir", dir,
		"--with-colons", "--list-keys", "Test User <test@example.com>",
	)
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("gpg --list-keys failed: %v\n%s", err, output)
	}

	var fp string
	for _, line := range bytes.Split(output, []byte("\n")) {
		fields := bytes.Split(line, []byte(":"))
		if len(fields) > 0 && string(fields[0]) == "fpr" {
			fp = string(fields[len(fields)-2])
			break
		}
	}
	if fp == "" {
		t.Fatalf("failed to parse fingerprint from GPG output:\n%s", output)
	}

	return fp
}
