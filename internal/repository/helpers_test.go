// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
)

var (
	gpgKeyBytes             = artifacts.GPGKey1Private
	gpgPubKeyBytes          = artifacts.GPGKey1Public
	gpgUnauthorizedKeyBytes = artifacts.GPGKey2Private
	rootKeyBytes            = artifacts.SSLibKey1Private
	rootPubKeyBytes         = artifacts.SSLibKey1Public
	targetsKeyBytes         = artifacts.SSLibKey2Private
	targetsPubKeyBytes      = artifacts.SSLibKey2Public
	rsaKeyBytes             = artifacts.SSHRSAPrivate
	ecdsaKeyBytes           = artifacts.SSHECDSAPrivate

	testCtx = context.Background()
)

func createTestRepositoryWithRoot(t *testing.T, location string) (*Repository, []byte) {
	t.Helper()

	var (
		repo *gitinterface.Repository
		err  error
	)

	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	if location == "" {
		tempDir := t.TempDir()
		repo = gitinterface.CreateTestGitRepository(t, tempDir, false)
	} else {
		repo = gitinterface.CreateTestGitRepository(t, location, false)
	}
	if err != nil {
		t.Fatal(err)
	}

	r := &Repository{r: repo}

	if err := r.InitializeRoot(testCtx, signer, false); err != nil {
		t.Fatal(err)
	}

	if err := policy.Apply(testCtx, repo, false); err != nil {
		t.Fatalf("failed to apply policy staging changes into policy, err = %s", err)
	}

	return r, rootKeyBytes
}

func createTestRepositoryWithPolicy(t *testing.T, location string) *Repository {
	t.Helper()

	r, keyBytes := createTestRepositoryWithRoot(t, location)

	rootSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(keyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	targetsSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	targetsPubKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.AddTopLevelTargetsKey(testCtx, rootSigner, targetsPubKey, false); err != nil {
		t.Fatal(err)
	}

	if err := r.InitializeTargets(testCtx, targetsSigner, policy.TargetsRoleName, false); err != nil {
		t.Fatal(err)
	}

	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	if err := r.AddDelegation(testCtx, targetsSigner, policy.TargetsRoleName, "protect-main", []*tuf.Key{gpgKey}, []string{"git:refs/heads/main"}, 1, false); err != nil {
		t.Fatal(err)
	}

	if err := policy.Apply(testCtx, r.r, false); err != nil {
		t.Fatalf("failed to apply policy staging changes into policy, err = %s", err)
	}

	return r
}
