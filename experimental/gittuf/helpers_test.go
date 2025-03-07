// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	trustpolicyopts "github.com/gittuf/gittuf/experimental/gittuf/options/trustpolicy"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/require"
)

var (
	gpgKeyBytes             = artifacts.GPGKey1Private
	gpgPubKeyBytes          = artifacts.GPGKey1Public
	gpgUnauthorizedKeyBytes = artifacts.GPGKey2Private
	rootKeyBytes            = artifacts.SSHRSAPrivate
	rootPubKeyBytes         = artifacts.SSHRSAPublicSSH
	targetsKeyBytes         = artifacts.SSHECDSAPrivate
	targetsPubKeyBytes      = artifacts.SSHECDSAPublicSSH
	rsaKeyBytes             = artifacts.SSHRSAPrivate
	ecdsaKeyBytes           = artifacts.SSHECDSAPrivate

	testCtx = context.Background()
)

func createTestRepositoryWithRoot(t *testing.T, location string) *Repository {
	t.Helper()

	signer := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	var repo *gitinterface.Repository
	if location == "" {
		tempDir := t.TempDir()
		repo = gitinterface.CreateTestGitRepository(t, tempDir, false)
	} else {
		repo = gitinterface.CreateTestGitRepository(t, location, false)
	}

	r := &Repository{r: repo}

	if err := r.InitializeRoot(testCtx, signer, false); err != nil {
		t.Fatal(err)
	}

	err := r.StagePolicy(testCtx, "", true, false)
	require.Nil(t, err)

	if err := policy.Apply(testCtx, repo, false); err != nil {
		t.Fatalf("failed to apply policy staging changes into policy, err = %s", err)
	}

	return r
}

func createTestRepositoryWithPolicy(t *testing.T, location string) *Repository {
	t.Helper()

	r := createTestRepositoryWithRoot(t, location)

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)

	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
	targetsPubKey := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

	if err := r.AddTopLevelTargetsKey(testCtx, rootSigner, targetsPubKey, false, trustpolicyopts.WithRSLEntry()); err != nil {
		t.Fatal(err)
	}

	if err := r.InitializeTargets(testCtx, targetsSigner, policy.TargetsRoleName, false, trustpolicyopts.WithRSLEntry()); err != nil {
		t.Fatal(err)
	}

	gpgKeyR, err := gpg.LoadGPGKeyFromBytes(gpgKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	gpgKey := tufv01.NewKeyFromSSLibKey(gpgKeyR)

	if err := r.AddPrincipalToTargets(testCtx, targetsSigner, policy.TargetsRoleName, []tuf.Principal{gpgKey}, false, trustpolicyopts.WithRSLEntry()); err != nil {
		t.Fatal(err)
	}

	if err := r.AddDelegation(testCtx, targetsSigner, policy.TargetsRoleName, "protect-main", []string{gpgKey.KeyID}, []string{"git:refs/heads/main"}, 1, false, trustpolicyopts.WithRSLEntry()); err != nil {
		t.Fatal(err)
	}

	if err := policy.Apply(testCtx, r.r, false); err != nil {
		t.Fatalf("failed to apply policy staging changes into policy, err = %s", err)
	}

	return r
}

func setupSSHKeysForSigning(t *testing.T, privateBytes, publicBytes []byte) *ssh.Signer {
	t.Helper()

	keysDir := t.TempDir()
	privKeyPath := filepath.Join(keysDir, "key")
	pubKeyPath := filepath.Join(keysDir, "key.pub")

	if err := os.WriteFile(privKeyPath, privateBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(pubKeyPath, publicBytes, 0o600); err != nil {
		t.Fatal(err)
	}

	signer, err := ssh.NewSignerFromFile(privKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	return signer
}
