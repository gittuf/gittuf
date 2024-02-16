// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"fmt"
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestClone(t *testing.T) {
	remoteTmpDir := t.TempDir()

	targetsPubKey, err := tuf.LoadKeyFromBytes(targetsPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	rootSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	targetsSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}

	remoteR, err := git.PlainInit(remoteTmpDir, true)
	if err != nil {
		t.Fatal(err)
	}
	remoteRepo := &Repository{r: remoteR}
	if err := remoteRepo.InitializeRoot(context.Background(), rootSigner, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.AddTopLevelTargetsKey(context.Background(), rootSigner, targetsPubKey, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.InitializeTargets(context.Background(), targetsSigner, policy.TargetsRoleName, false); err != nil {
		t.Fatal(err)
	}
	emptyTreeHash, err := gitinterface.WriteTree(remoteRepo.r, nil)
	if err != nil {
		t.Fatal(err)
	}
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"
	commitID, err := gitinterface.Commit(remoteRepo.r, emptyTreeHash, refName, "Initial commit", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.r.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.ReferenceName(refName))); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.r.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(anotherRefName), commitID)); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.RecordRSLEntryForReference(anotherRefName, false); err != nil {
		t.Fatal(err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	remoteRSLRef, err := remoteRepo.r.Reference(plumbing.ReferenceName(rsl.Ref), true)
	if err != nil {
		t.Fatal(err)
	}
	remotePolicyRef, err := remoteRepo.r.Reference(plumbing.ReferenceName(policy.PolicyRef), true)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("successful clone without specifying dir", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(context.Background(), remoteTmpDir, "", "", []*tuf.Key{})
		assert.Nil(t, err)
		head, err := repo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head.Hash())

		localRSLRef, err := repo.r.Reference(plumbing.ReferenceName(rsl.Ref), true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remoteRSLRef.Hash(), localRSLRef.Hash())
		localPolicyRef, err := repo.r.Reference(plumbing.ReferenceName(policy.PolicyRef), true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remotePolicyRef.Hash(), localPolicyRef.Hash())
	})

	t.Run("successful clone with dir", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		dirName := "myRepo"
		repo, err := Clone(context.Background(), remoteTmpDir, dirName, "", []*tuf.Key{})
		assert.Nil(t, err)
		head, err := repo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head.Hash())

		dirInfo, err := os.Stat(dirName)
		assert.Nil(t, err)
		assert.True(t, dirInfo.IsDir())

		localRSLRef, err := repo.r.Reference(plumbing.ReferenceName(rsl.Ref), true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remoteRSLRef.Hash(), localRSLRef.Hash())
		localPolicyRef, err := repo.r.Reference(plumbing.ReferenceName(policy.PolicyRef), true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remotePolicyRef.Hash(), localPolicyRef.Hash())
	})

	t.Run("successful clone without specifying dir, with non-HEAD initial branch", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(context.Background(), remoteTmpDir, "", anotherRefName, []*tuf.Key{})
		assert.Nil(t, err)
		head, err := repo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head.Hash())
		assert.Equal(t, plumbing.ReferenceName(anotherRefName), head.Name())

		localRSLRef, err := repo.r.Reference(plumbing.ReferenceName(rsl.Ref), true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remoteRSLRef.Hash(), localRSLRef.Hash())
		localPolicyRef, err := repo.r.Reference(plumbing.ReferenceName(policy.PolicyRef), true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remotePolicyRef.Hash(), localPolicyRef.Hash())
	})

	t.Run("unsuccessful clone when unspecified dir already exists", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		_, err = Clone(context.Background(), remoteTmpDir, "", "", []*tuf.Key{})
		assert.Nil(t, err)

		_, err = Clone(context.Background(), remoteTmpDir, "", "", []*tuf.Key{})
		assert.ErrorIs(t, err, ErrDirExists)
	})

	t.Run("unsuccessful clone when specified dir already exists", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		dirName := "myRepo"
		if err := os.Mkdir(dirName, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err = Clone(context.Background(), remoteTmpDir, dirName, "", []*tuf.Key{})
		assert.ErrorIs(t, err, ErrDirExists)
	})

	t.Run("successful clone without specifying dir, with trailing slashes in repository path", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(context.Background(), remoteTmpDir+"//", "", "", []*tuf.Key{})
		assert.Nil(t, err)
		head, err := repo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head.Hash())

		localRSLRef, err := repo.r.Reference(plumbing.ReferenceName(rsl.Ref), true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remoteRSLRef.Hash(), localRSLRef.Hash())
		localPolicyRef, err := repo.r.Reference(plumbing.ReferenceName(policy.PolicyRef), true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remotePolicyRef.Hash(), localPolicyRef.Hash())
	})

	t.Run("successful clone without specifying dir, with expected root keys", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		publicKey, err := tuf.LoadKeyFromBytes(rootKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		repo, err := Clone(context.Background(), remoteTmpDir, "", "", []*tuf.Key{publicKey})
		assert.Nil(t, err)
		head, err := repo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head.Hash())

		localRSLRef, err := repo.r.Reference(plumbing.ReferenceName(rsl.Ref), true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remoteRSLRef.Hash(), localRSLRef.Hash())
		localPolicyRef, err := repo.r.Reference(plumbing.ReferenceName(policy.PolicyRef), true)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remotePolicyRef.Hash(), localPolicyRef.Hash())
	})

	t.Run("unsuccessful clone without specifying dir, with expected root keys not equaling root keys", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		expectedPublicKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
		if err != nil {
			t.Fatal(err)
		}
		actualPublicKey, err := tuf.LoadKeyFromBytes(rootKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		_, err = Clone(context.Background(), remoteTmpDir, "", "", []*tuf.Key{expectedPublicKey})
		targetErr := fmt.Errorf(ClonedAndExpectedKeysDoNotMatch, []string{expectedPublicKey.KeyID}, []string{actualPublicKey.KeyID})
		assert.ErrorIs(t, err, ErrCloningRepository, targetErr)
	})
}
