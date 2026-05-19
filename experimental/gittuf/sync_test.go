// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"os"
	"testing"

	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestClone(t *testing.T) {
	remoteTmpDir := t.TempDir()

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
	targetsPubKey := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

	remoteR := gitinterface.CreateTestGitRepository(t, remoteTmpDir, true)
	remoteRepo := &Repository{r: remoteR}
	treeBuilder := gitinterface.NewTreeBuilder(remoteR)
	emptyTreeHash, err := treeBuilder.WriteTreeFromEntries(nil)
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
	if err := remoteRepo.StagePolicy(testCtx, "", true, false); err != nil {
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
	if err := remoteRepo.RecordRSLEntryForReference(testCtx, refName, false, rslopts.WithRecordLocalOnly()); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.r.SetReference(anotherRefName, commitID); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.RecordRSLEntryForReference(testCtx, anotherRefName, false, rslopts.WithRecordLocalOnly()); err != nil {
		t.Fatal(err)
	}

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}

	t.Run("successful clone without specifying dir, bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(testCtx, remoteTmpDir, "", "", nil, true)
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

	t.Run("successful clone with dir, bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		dirName := "myRepo"
		repo, err := Clone(testCtx, remoteTmpDir, dirName, "", nil, true)
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

	t.Run("successful clone without specifying dir, with non-HEAD initial branch, bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(testCtx, remoteTmpDir, "", anotherRefName, nil, true)
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

	t.Run("unsuccessful clone when unspecified dir already exists, bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		_, err = Clone(testCtx, remoteTmpDir, "", "", nil, true)
		assert.Nil(t, err)

		_, err = Clone(testCtx, remoteTmpDir, "", "", nil, true)
		assert.ErrorIs(t, err, ErrDirExists)
	})

	t.Run("unsuccessful clone when specified dir already exists, bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		dirName := "myRepo"
		if err := os.Mkdir(dirName, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err = Clone(testCtx, remoteTmpDir, dirName, "", nil, true)
		assert.ErrorIs(t, err, ErrDirExists)
	})

	t.Run("successful clone without specifying dir, with trailing slashes in repository path, bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(testCtx, remoteTmpDir+"//", "", "", nil, true)
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

	t.Run("successful clone without specifying dir, with multiple expected root keys, bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		rootPublicKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
		targetsPublicKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

		repo, err := Clone(testCtx, remoteTmpDir, "", "", []tuf.Principal{targetsPublicKey, rootPublicKey}, true)
		assert.Nil(t, err)

		head, err := repo.r.GetReference("HEAD")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head)

		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, policy.PolicyRef)
	})

	t.Run("unsuccessful clone without specifying dir, with expected root keys not equaling root keys, bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		badPublicKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}
		badPublicKey := tufv01.NewKeyFromSSLibKey(badPublicKeyR)

		rootPublicKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))

		_, err = Clone(testCtx, remoteTmpDir, "", "", []tuf.Principal{rootPublicKey, badPublicKey}, true)
		assert.ErrorIs(t, ErrExpectedRootKeysDoNotMatch, err)
	})

	t.Run("successful clone without specifying dir, not bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(testCtx, remoteTmpDir, "", "", nil, false)
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

	t.Run("successful clone with dir, not bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		dirName := "myRepo"
		repo, err := Clone(testCtx, remoteTmpDir, dirName, "", nil, false)
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

	t.Run("successful clone without specifying dir, with non-HEAD initial branch, not bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(testCtx, remoteTmpDir, "", anotherRefName, nil, false)
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

	t.Run("unsuccessful clone when unspecified dir already exists, not bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		_, err = Clone(testCtx, remoteTmpDir, "", "", nil, false)
		assert.Nil(t, err)

		_, err = Clone(testCtx, remoteTmpDir, "", "", nil, false)
		assert.ErrorIs(t, err, ErrDirExists)
	})

	t.Run("unsuccessful clone when specified dir already exists, not bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		dirName := "myRepo"
		if err := os.Mkdir(dirName, 0o755); err != nil {
			t.Fatal(err)
		}
		_, err = Clone(testCtx, remoteTmpDir, dirName, "", nil, false)
		assert.ErrorIs(t, err, ErrDirExists)
	})

	t.Run("successful clone without specifying dir, with trailing slashes in repository path, not bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		repo, err := Clone(testCtx, remoteTmpDir+"//", "", "", nil, false)
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

	t.Run("successful clone without specifying dir, with multiple expected root keys, not bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		rootPublicKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
		targetsPublicKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targetsPubKeyBytes))

		repo, err := Clone(testCtx, remoteTmpDir, "", "", []tuf.Principal{targetsPublicKey, rootPublicKey}, false)
		assert.Nil(t, err)

		head, err := repo.r.GetReference("HEAD")
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head)

		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, rsl.Ref)
		assertLocalAndRemoteRefsMatch(t, repo.r, remoteRepo.r, policy.PolicyRef)
	})

	t.Run("unsuccessful clone without specifying dir, with expected root keys not equaling root keys, not bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		badPublicKeyR, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
		if err != nil {
			t.Fatal(err)
		}
		badPublicKey := tufv01.NewKeyFromSSLibKey(badPublicKeyR)

		rootPublicKey := tufv01.NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))

		_, err = Clone(testCtx, remoteTmpDir, "", "", []tuf.Principal{rootPublicKey, badPublicKey}, false)
		assert.ErrorIs(t, ErrExpectedRootKeysDoNotMatch, err)
	})
}
