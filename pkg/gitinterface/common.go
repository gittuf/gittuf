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

// ObjectFormat identifies the hash algorithm a Git repository uses for its
// object IDs.
type ObjectFormat string

const (
	ObjectFormatSHA1   ObjectFormat = "sha1"
	ObjectFormatSHA256 ObjectFormat = "sha256"
)

const (
	testName  = "Jane Doe"
	testEmail = "jane.doe@example.com"
)

var (
	testClock = clockwork.NewFakeClockAt(time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC))
)

type testRepositoryOptions struct {
	objectFormat ObjectFormat
}

// TestRepositoryOption configures the repository created by
// CreateTestGitRepository.
type TestRepositoryOption func(o *testRepositoryOptions)

// WithObjectFormat creates the test repository using the specified object
// format (hash algorithm).
func WithObjectFormat(objectFormat ObjectFormat) TestRepositoryOption {
	return func(o *testRepositoryOptions) {
		o.objectFormat = objectFormat
	}
}

// WithSHA256Format creates the test repository using the SHA-256 object
// format.
func WithSHA256Format() TestRepositoryOption {
	return WithObjectFormat(ObjectFormatSHA256)
}

// CreateTestGitRepository creates a Git repository in the specified directory,
// using the SHA-1 object format unless overridden via options. This is meant
// to be used by tests across gittuf packages. This helper also sets up an
// SSH RSA signing key that can be used to create reproducible commits.
func CreateTestGitRepository(t *testing.T, dir string, bare bool, opts ...TestRepositoryOption) *Repository {
	t.Helper()

	repo, err := createTestGitRepository(dir, t.TempDir(), bare, opts...)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func createTestGitRepository(dir, signingKeysDir string, bare bool, opts ...TestRepositoryOption) (*Repository, error) {
	options := &testRepositoryOptions{objectFormat: ObjectFormatSHA1}
	for _, fn := range opts {
		fn(options)
	}

	repo, err := newTestRepository(dir, bare, options.objectFormat)
	if err != nil {
		return nil, err
	}

	// Set up author / committer identity
	if err := repo.SetGitConfig("user.name", testName); err != nil {
		return nil, err
	}
	if err := repo.SetGitConfig("user.email", testEmail); err != nil {
		return nil, err
	}

	// Set up signing via SSH key
	if err := writeSigningKeys(signingKeysDir); err != nil {
		return nil, err
	}

	if err := repo.SetGitConfig("user.signingkey", filepath.Join(signingKeysDir, "key.pub")); err != nil {
		return nil, err
	}
	if err := repo.SetGitConfig("gpg.format", "ssh"); err != nil {
		return nil, err
	}

	return repo, nil
}

func setupRepository(t *testing.T, dir string, bare bool, objectFormat ObjectFormat) *Repository {
	t.Helper()

	repo, err := newTestRepository(dir, bare, objectFormat)
	if err != nil {
		t.Fatal(err)
	}
	return repo
}

func newTestRepository(dir string, bare bool, objectFormat ObjectFormat) (*Repository, error) {
	var gitDirPath string
	args := []string{"init"}
	if bare {
		args = append(args, "--bare")
		gitDirPath = dir
	} else {
		gitDirPath = filepath.Join(dir, ".git")
	}
	args = append(args, "--object-format", string(objectFormat))
	args = append(args, "-b", "main")
	args = append(args, dir)

	cmd := exec.Command(binary, args...)
	if err := cmd.Run(); err != nil {
		return nil, err
	}

	repo := &Repository{gitDirPath: gitDirPath, clock: testClock}

	format, err := repo.readObjectFormat()
	if err != nil {
		return nil, err
	}
	repo.objectFormat = format

	return repo, nil
}

func writeSigningKeys(dir string) error {
	sshPrivateKey := artifacts.SSHRSAPrivate
	sshPublicKey := artifacts.SSHRSAPublicSSH

	privateKeyPath := filepath.Join(dir, "key")
	publicKeyPath := filepath.Join(dir, "key.pub")

	if err := os.WriteFile(privateKeyPath, sshPrivateKey, 0o600); err != nil {
		return err
	}

	if err := os.WriteFile(publicKeyPath, sshPublicKey, 0o600); err != nil {
		return err
	}

	return nil
}
