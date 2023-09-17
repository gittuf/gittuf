package repository

import (
	"context"
	_ "embed"
	"fmt"
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/config"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

//go:embed test-data/root
var rootKeyBytes []byte

//go:embed test-data/targets
var targetsKeyBytes []byte

//go:embed test-data/targets.pub
var targetsPubKeyBytes []byte

func TestClone(t *testing.T) {
	remoteTmpDir := t.TempDir()

	remoteR, err := git.PlainInit(remoteTmpDir, true)
	if err != nil {
		t.Fatal(err)
	}
	remoteRepo := &Repository{r: remoteR}
	if err := remoteRepo.InitializeRoot(context.Background(), rootKeyBytes, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.AddTopLevelTargetsKey(context.Background(), rootKeyBytes, targetsPubKeyBytes, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.InitializeTargets(context.Background(), targetsKeyBytes, policy.TargetsRoleName, false); err != nil {
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

	remoteRSLRef, err := remoteRepo.r.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
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

		repo, err := Clone(context.Background(), remoteTmpDir, "", "")
		assert.Nil(t, err)
		head, err := repo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head.Hash())

		localRSLRef, err := repo.r.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
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
		repo, err := Clone(context.Background(), remoteTmpDir, dirName, "")
		assert.Nil(t, err)
		head, err := repo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head.Hash())

		dirInfo, err := os.Stat(dirName)
		assert.Nil(t, err)
		assert.True(t, dirInfo.IsDir())

		localRSLRef, err := repo.r.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
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

		repo, err := Clone(context.Background(), remoteTmpDir, "", anotherRefName)
		assert.Nil(t, err)
		head, err := repo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, commitID, head.Hash())
		assert.Equal(t, plumbing.ReferenceName(anotherRefName), head.Name())

		localRSLRef, err := repo.r.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
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

		_, err = Clone(context.Background(), remoteTmpDir, "", "")
		assert.Nil(t, err)

		_, err = Clone(context.Background(), remoteTmpDir, "", "")
		assert.ErrorIs(t, err, ErrDirExists)
	})

	t.Run("unsuccessful clone when specified dir already exists", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		dirName := "myRepo"
		if err := os.Mkdir(dirName, 0755); err != nil {
			t.Fatal(err)
		}
		_, err = Clone(context.Background(), remoteTmpDir, dirName, "")
		assert.ErrorIs(t, err, ErrDirExists)
	})
}

func TestPush(t *testing.T) {
	remoteName := "origin"
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"
	refNameTyped := plumbing.ReferenceName(refName)
	anotherRefNameTyped := plumbing.ReferenceName(anotherRefName)
	rslRefNameTyped := plumbing.ReferenceName(rsl.RSLRef)
	policyRefNameTyped := plumbing.ReferenceName(policy.PolicyRef)

	repoLocal := createTestRepositoryWithPolicy(t)

	// Create tmp dir for destination repo so we have a URL for it
	tmpDir, err := os.MkdirTemp("", "gittuf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(tmpDir) //nolint:errcheck

	repoRemote, err := git.PlainInit(tmpDir, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = repoLocal.r.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{tmpDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assert that the remote repo does not contain the main branch or the
	// gittuf refs
	_, err = repoRemote.Reference(refNameTyped, true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	_, err = repoRemote.Reference(rslRefNameTyped, true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	_, err = repoRemote.Reference(policyRefNameTyped, true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)

	// Create a test commit and its RSL entry
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, repoLocal.r, refName, 1)
	entry := rsl.NewEntry(refName, commitIDs[0])
	common.CreateTestRSLEntryCommit(t, repoLocal.r, entry)
	if err := repoLocal.r.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, refNameTyped)); err != nil {
		t.Fatal(err)
	}

	// gittuf namespaces are not explicitly named here for Push
	err = repoLocal.Push(context.Background(), remoteName, refName)
	assert.Nil(t, err)

	localRef, err := repoLocal.r.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	remoteRef, err := repoRemote.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, localRef.Hash(), remoteRef.Hash())

	localRSLRef, err := repoLocal.r.Reference(rslRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	remoteRSLRef, err := repoRemote.Reference(rslRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, localRSLRef.Hash(), remoteRSLRef.Hash())

	localPolicyRef, err := repoLocal.r.Reference(policyRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	remotePolicyRef, err := repoRemote.Reference(policyRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, localPolicyRef.Hash(), remotePolicyRef.Hash())

	// Verify remote doesn't have the second ref
	_, err = repoRemote.Reference(anotherRefNameTyped, true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)

	// Initialize second ref and add another commit to first ref
	if err := repoLocal.r.Storer.SetReference(plumbing.NewHashReference(anotherRefNameTyped, localRef.Hash())); err != nil {
		t.Fatal(err)
	}

	entry = rsl.NewEntry(anotherRefName, commitIDs[0])
	common.CreateTestRSLEntryCommit(t, repoLocal.r, entry)

	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, repoLocal.r, refName, 1)

	entry = rsl.NewEntry(refName, commitIDs[0])
	common.CreateTestRSLEntryCommit(t, repoLocal.r, entry)

	// Push both
	err = repoLocal.Push(context.Background(), remoteName, refName, anotherRefName)
	assert.Nil(t, err)

	localRef, err = repoLocal.r.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	remoteRef, err = repoRemote.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, localRef.Hash(), remoteRef.Hash())

	localAnotherRef, err := repoLocal.r.Reference(anotherRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	remoteAnotherRef, err := repoRemote.Reference(anotherRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, localAnotherRef.Hash(), remoteAnotherRef.Hash())

	localRSLRef, err = repoLocal.r.Reference(rslRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	remoteRSLRef, err = repoRemote.Reference(rslRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, localRSLRef.Hash(), remoteRSLRef.Hash())

	localPolicyRef, err = repoLocal.r.Reference(policyRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	remotePolicyRef, err = repoRemote.Reference(policyRefNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, localPolicyRef.Hash(), remotePolicyRef.Hash())
}

func TestPull(t *testing.T) {
	remoteTmpDir, err := os.MkdirTemp("", "gittuf")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(remoteTmpDir) //nolint:errcheck

	remoteR, err := git.PlainInit(remoteTmpDir, true)
	if err != nil {
		t.Fatal(err)
	}
	remoteRepo := &Repository{r: remoteR}
	if err := remoteRepo.InitializeRoot(context.Background(), rootKeyBytes, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.AddTopLevelTargetsKey(context.Background(), rootKeyBytes, targetsPubKeyBytes, false); err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.InitializeTargets(context.Background(), targetsKeyBytes, policy.TargetsRoleName, false); err != nil {
		t.Fatal(err)
	}
	emptyTreeHash, err := gitinterface.WriteTree(remoteRepo.r, nil)
	if err != nil {
		t.Fatal(err)
	}
	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"
	remoteName := "origin"
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

	remoteRSLRef, err := remoteRepo.r.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}
	remotePolicyRef, err := remoteRepo.r.Reference(plumbing.ReferenceName(policy.PolicyRef), true)
	if err != nil {
		t.Fatal(err)
	}

	// repository.Clone is an option but that needs a dir
	localR, err := gitinterface.CloneAndFetchToMemory(context.Background(), remoteTmpDir, "", []string{rsl.RSLRef, policy.PolicyRef}, false)
	if err != nil {
		t.Fatal(err)
	}
	localRepo := &Repository{r: localR}

	localRSLRef, err := localRepo.r.Reference(plumbing.ReferenceName(rsl.RSLRef), true)
	if err != nil {
		t.Fatal(err)
	}
	localPolicyRef, err := localRepo.r.Reference(plumbing.ReferenceName(policy.PolicyRef), true)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, remoteRSLRef.Hash(), localRSLRef.Hash())
	assert.Equal(t, remotePolicyRef.Hash(), localPolicyRef.Hash())

	// Make remote changes
	newCommitID, err := gitinterface.Commit(remoteRepo.r, emptyTreeHash, refName, "Another commit", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
		t.Fatal(err)
	}

	err = localRepo.Pull(context.Background(), remoteName, refName)
	assert.Nil(t, err)

	localRef, err := localRepo.r.Reference(plumbing.ReferenceName(refName), true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, newCommitID, localRef.Hash())

	// Assert second ref isn't in local
	_, err = localRepo.r.Reference(plumbing.ReferenceName(anotherRefName), true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)

	err = localRepo.Pull(context.Background(), remoteName, anotherRefName)
	assert.Nil(t, err)

	remoteAnotherRef, err := remoteRepo.r.Reference(plumbing.ReferenceName(anotherRefName), true)
	if err != nil {
		t.Fatal(err)
	}
	localAnotherRef, err := localRepo.r.Reference(plumbing.ReferenceName(anotherRefName), true)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, remoteAnotherRef.Hash(), localAnotherRef.Hash())
}

func TestGetLocalDirName(t *testing.T) {
	tests := map[string]string{
		"unix dir":                            "/tmp/dir/gittuf",
		"unix dir with .git":                  "/tmp/dir/gittuf.git",
		"windows dir":                         `D:\\Documents\gittuf`,
		"windows dir with .git":               `D:\\Documents\gittuf.git`,
		"URL":                                 "https://github.com/gittuf/gittuf",
		"URL with .git":                       "https://github.com/gittuf/gittuf.git",
		"SSH URL":                             "git@github.com:gittuf/gittuf",
		"SSH URL with .git":                   "git@github.com:gittuf/gittuf.git",
		"SSH URL with protocol":               "ssh:git@github.com:gittuf/gittuf",
		"SSH URL with protocol and with .git": "ssh:git@github.com:gittuf/gittuf.git",
	}
	expectedDirName := "gittuf"

	for name, url := range tests {
		dirName := getLocalDirName(url)
		assert.Equal(t, expectedDirName, dirName, fmt.Sprintf("unexpected result in test '%s", name))
	}
}
