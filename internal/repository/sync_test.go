// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/tuf"
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

	remoteR := gitinterface.CreateTestGitRepository(t, remoteTmpDir, true)
	remoteRepo := &Repository{r: remoteR}
	treeBuilder := gitinterface.NewReplacementTreeBuilder(remoteR)
	emptyTreeHash, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
	if err != nil {
		t.Fatal(err)
	}

	if err := remoteRepo.InitializeRoot(testCtx, rootSigner, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.AddRootKey(testCtx, rootSigner, targetsPubKey, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.AddTopLevelTargetsKey(testCtx, rootSigner, targetsPubKey, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.SignRoot(testCtx, targetsSigner, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.InitializeTargets(testCtx, targetsSigner, policy.TargetsRoleName, false); err != nil {
		t.Fatal(err)
	}
	if err := policy.Apply(testCtx, remoteRepo.r, false); err != nil {
		t.Fatal(err)
	}

	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"
	commitID, err := remoteRepo.r.Commit(emptyTreeHash, refName, "Initial commit", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.r.SetReference(anotherRefName, commitID); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.RecordRSLEntryForReference(anotherRefName, false); err != nil {
		t.Fatal(err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("successful clone without specifying dir", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(testCtx, remoteTmpDir, "", "", nil)
		assert.Nil(t, err)
		head, err := repo.r.GetSymbolicReferenceTarget("HEAD")
		if err != nil {
			t.Fatal(err)
		}
		headID, err := repo.r.GetReference(head)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, headID)

		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, policy.PolicyRef)
	})

	t.Run("successful clone with dir", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		dirName := "myRepo"
		repo, err := Clone(testCtx, remoteTmpDir, dirName, "", nil)
		assert.Nil(t, err)
		head, err := repo.r.GetSymbolicReferenceTarget("HEAD")
		if err != nil {
			t.Fatal(err)
		}
		headID, err := repo.r.GetReference(head)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, headID)

		dirInfo, err := os.Stat(dirName)
		assert.Nil(t, err)
		assert.True(t, dirInfo.IsDir())

		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, policy.PolicyRef)
	})

	t.Run("successful clone without specifying dir, with non-HEAD initial branch", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(testCtx, remoteTmpDir, "", anotherRefName, nil)
		assert.Nil(t, err)
		head, err := repo.r.GetSymbolicReferenceTarget("HEAD")
		if err != nil {
			t.Fatal(err)
		}
		headID, err := repo.r.GetReference(head)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, headID)
		assert.Equal(t, anotherRefName, head)

		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, policy.PolicyRef)
	})

	t.Run("unsuccessful clone when unspecified dir already exists", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		_, err = Clone(testCtx, remoteTmpDir, "", "", nil)
		assert.Nil(t, err)

		_, err = Clone(testCtx, remoteTmpDir, "", "", nil)
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
		_, err = Clone(testCtx, remoteTmpDir, dirName, "", nil)
		assert.ErrorIs(t, err, ErrDirExists)
	})

	t.Run("successful clone without specifying dir, with trailing slashes in repository path", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(testCtx, remoteTmpDir+"//", "", "", nil)
		assert.Nil(t, err)
		head, err := repo.r.GetSymbolicReferenceTarget("HEAD")
		if err != nil {
			t.Fatal(err)
		}
		headID, err := repo.r.GetReference(head)
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, headID)

		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, policy.PolicyRef)
	})

	t.Run("successful clone without specifying dir, with multiple expected root keys", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		rootPublicKey, err := tuf.LoadKeyFromBytes(rootKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		targetsPublicKey, err := tuf.LoadKeyFromBytes(targetsKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		repo, err := Clone(testCtx, remoteTmpDir, "", "", []*tuf.Key{targetsPublicKey, rootPublicKey})
		assert.Nil(t, err)

		head, err := repo.r.GetReference("HEAD")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head)

		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, policy.PolicyRef)
	})

	t.Run("unsuccessful clone without specifying dir, with expected root keys not equaling root keys", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		badPublicKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		rootPublicKey, err := tuf.LoadKeyFromBytes(rootKeyBytes)
		if err != nil {
			t.Fatal(err)
		}

		_, err = Clone(testCtx, remoteTmpDir, "", "", []*tuf.Key{rootPublicKey, badPublicKey})
		assert.ErrorIs(t, ErrExpectedRootKeysDoNotMatch, err)
	})
}
