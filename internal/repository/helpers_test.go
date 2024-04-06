// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"testing"

	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/storage/memory"
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
		repo *git.Repository
		err  error
	)

	signer, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	if location == "" {
		repo, err = git.Init(memory.NewStorage(), memfs.New())
	} else {
		repo, err = git.PlainInit(location, true)
	}
	if err != nil {
		t.Fatal(err)
	}

	r := &Repository{r: repo}

	if err := r.InitializeRoot(testCtx, signer, false); err != nil {
		t.Fatal(err)
	}

	if err := policy.Apply(testCtx, repo, []string{"."}, false); err != nil {
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

	if err := policy.Apply(testCtx, r.r, []string{"."}, false); err != nil {
		t.Fatalf("failed to apply policy staging changes into policy, err = %s", err)
	}

	return r
}
