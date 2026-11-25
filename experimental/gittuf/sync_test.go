// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"net/url"
	"os"
	"path/filepath"
	"testing"

	rslopts "github.com/gittuf/gittuf/experimental/gittuf/options/rsl"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
)

func TestClone(t *testing.T) {
	remoteTmpDir := t.TempDir()

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)
	targetsPubKey := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

	remoteR := gitinterface.CreateTestGitRepository(t, remoteTmpDir, true)
	remoteRepo := &Repository{r: newStorer(remoteR)}
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

	keysDir := t.TempDir()
	rootPubKeyPath := filepath.Join(keysDir, "root.pub")
	if err := os.WriteFile(rootPubKeyPath, rootPubKeyBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	targetsPubKeyPath := filepath.Join(keysDir, "targets.pub")
	if err := os.WriteFile(targetsPubKeyPath, targetsPubKeyBytes, 0o600); err != nil {
		t.Fatal(err)
	}
	gpgKeyRef := GPGKeyPrefix + "157507bbe151e378ce8126c1dcfe043cdd2db96e"

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

	t.Run("unsuccessful clone when dir cannot be guessed from the url, bare", func(t *testing.T) {
		_, err := Clone(testCtx, "/", "", "", nil, true)
		assert.ErrorIs(t, err, ErrCloningRepository)
		assert.ErrorIs(t, err, ErrCannotDetermineCloneDir)
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

		repo, err := Clone(testCtx, remoteTmpDir, "", "", []string{targetsPubKeyPath, rootPubKeyPath}, true)
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

		gpg.SetupTestGPGHomeDir(t, gpgKeyBytes)

		_, err := Clone(testCtx, remoteTmpDir, "", "", []string{rootPubKeyPath, gpgKeyRef}, true)
		assert.ErrorIs(t, ErrExpectedRootKeysDoNotMatch, err)
	})

	t.Run("unsuccessful clone with error loading expected root keys, bare", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		// Point gpg.program at a program that doesn't exist
		configPath := filepath.Join(t.TempDir(), "gitconfig")
		if err := os.WriteFile(configPath, []byte("[gpg]\n\tprogram = nonexistent-gpg\n"), 0o600); err != nil {
			t.Fatal(err)
		}
		t.Setenv("GIT_CONFIG_GLOBAL", configPath)

		dir := "clone-gpg-program"
		_, err = Clone(testCtx, remoteTmpDir, dir, "", []string{gpgKeyRef}, true)
		assert.ErrorIs(t, err, ErrCloningRepository)
		assert.ErrorContains(t, err, "nonexistent-gpg")

		// The clone is removed because the root key couldn't be loaded
		_, statErr := os.Stat(dir)
		assert.True(t, os.IsNotExist(statErr))
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

		repo, err := Clone(testCtx, remoteTmpDir, "", "", []string{targetsPubKeyPath, rootPubKeyPath}, false)
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

		gpg.SetupTestGPGHomeDir(t, gpgKeyBytes)

		_, err := Clone(testCtx, remoteTmpDir, "", "", []string{rootPubKeyPath, gpgKeyRef}, false)
		assert.ErrorIs(t, ErrExpectedRootKeysDoNotMatch, err)
	})
}

func TestDetermineCloneDir(t *testing.T) {
	tests := map[string]struct {
		remoteURL   string
		expectedDir string
		expectedErr error
	}{
		"https url": {
			remoteURL:   "https://github.com/gittuf/gittuf",
			expectedDir: "gittuf",
		},
		"https url with .git suffix": {
			remoteURL:   "https://github.com/gittuf/gittuf.git",
			expectedDir: "gittuf",
		},
		"ssh url with port": {
			remoteURL:   "ssh://git@example.com:2222/gittuf/gittuf.git",
			expectedDir: "gittuf",
		},
		"scp-like url": {
			remoteURL:   "git@github.com:gittuf/gittuf.git",
			expectedDir: "gittuf",
		},
		"scp-like url without slash": {
			remoteURL:   "example.com:gittuf.git",
			expectedDir: "gittuf",
		},
		"local path": {
			remoteURL:   "/tmp/gittuf",
			expectedDir: "gittuf",
		},
		"local path with trailing slashes": {
			remoteURL:   "/tmp/gittuf//",
			expectedDir: "gittuf",
		},
		"local path to .git directory": {
			remoteURL:   "/tmp/gittuf/.git",
			expectedDir: "gittuf",
		},
		"local path to .git directory with trailing slash": {
			remoteURL:   "/tmp/gittuf/.git/",
			expectedDir: "gittuf",
		},
		"file url to .git directory": {
			remoteURL:   "file:///tmp/gittuf/.git",
			expectedDir: "gittuf",
		},
		"windows path": {
			remoteURL:   `C:\Users\jane\gittuf`,
			expectedDir: "gittuf",
		},
		"windows path with forward slashes": {
			remoteURL:   "C:/Users/jane/gittuf",
			expectedDir: "gittuf",
		},
		"url without path": {
			remoteURL:   "https://example.com",
			expectedDir: "example.com",
		},
		"url without path with port": {
			remoteURL:   "https://example.com:8080/",
			expectedDir: "example.com",
		},
		"root path": {
			remoteURL:   "/",
			expectedErr: ErrCannotDetermineCloneDir,
		},
		"current directory": {
			remoteURL:   ".",
			expectedErr: ErrCannotDetermineCloneDir,
		},
		"root .git directory": {
			remoteURL:   "/.git",
			expectedErr: ErrCannotDetermineCloneDir,
		},
		"file url without path": {
			remoteURL:   "file:///",
			expectedErr: ErrCannotDetermineCloneDir,
		},
		"path ending in parent directory": {
			remoteURL:   "/tmp/..",
			expectedErr: ErrCannotDetermineCloneDir,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			dir, err := determineCloneDir(test.remoteURL)
			if test.expectedErr != nil {
				assert.ErrorIs(t, err, test.expectedErr)
				return
			}

			assert.NoError(t, err)
			assert.Equal(t, test.expectedDir, dir)
		})
	}

	t.Run("invalid url", func(t *testing.T) {
		_, err := determineCloneDir("https://example.com/%zz")
		var urlErr *url.Error
		assert.ErrorAs(t, err, &urlErr)
	})
}
