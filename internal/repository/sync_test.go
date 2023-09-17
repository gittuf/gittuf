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

	remoteRepo := initializeTestRepositoryInDir(t, remoteTmpDir)

	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"

	recordChangesInTestRepository(t, remoteRepo, refName, []string{anotherRefName})

	remoteHead, err := remoteRepo.r.Head()
	if err != nil {
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

		localRepo, err := Clone(context.Background(), remoteTmpDir, "", "")
		assert.Nil(t, err)
		head, err := localRepo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remoteHead.Hash(), head.Hash())

		assertRefsEqual(t, rsl.RSLRef, remoteRepo, localRepo)
		assertRefsEqual(t, policy.PolicyRef, remoteRepo, localRepo)
	})

	t.Run("successful clone with dir", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		dirName := "myRepo"
		localRepo, err := Clone(context.Background(), remoteTmpDir, dirName, "")
		assert.Nil(t, err)
		head, err := localRepo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remoteHead.Hash(), head.Hash())

		dirInfo, err := os.Stat(dirName)
		assert.Nil(t, err)
		assert.True(t, dirInfo.IsDir())

		assertRefsEqual(t, rsl.RSLRef, remoteRepo, localRepo)
		assertRefsEqual(t, policy.PolicyRef, remoteRepo, localRepo)
	})

	t.Run("successful clone without specifying dir, with non-HEAD initial branch", func(t *testing.T) {
		localTmpDir := t.TempDir()

		if err := os.Chdir(localTmpDir); err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck

		localRepo, err := Clone(context.Background(), remoteTmpDir, "", anotherRefName)
		assert.Nil(t, err)
		head, err := localRepo.r.Head()
		if err != nil {
			t.Fatal(err)
		}
		assert.Equal(t, remoteHead.Hash(), head.Hash())
		assert.Equal(t, plumbing.ReferenceName(anotherRefName), head.Name())

		assertRefsEqual(t, rsl.RSLRef, remoteRepo, localRepo)
		assertRefsEqual(t, policy.PolicyRef, remoteRepo, localRepo)
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
	tmpDir := t.TempDir()

	repoRemoteR, err := git.PlainInit(tmpDir, true)
	if err != nil {
		t.Fatal(err)
	}
	repoRemote := &Repository{r: repoRemoteR}
	_, err = repoLocal.r.CreateRemote(&config.RemoteConfig{
		Name: remoteName,
		URLs: []string{tmpDir},
	})
	if err != nil {
		t.Fatal(err)
	}

	// Assert that the remote repo does not contain the main branch or the
	// gittuf refs
	_, err = repoRemote.r.Reference(refNameTyped, true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	_, err = repoRemote.r.Reference(rslRefNameTyped, true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)
	_, err = repoRemote.r.Reference(policyRefNameTyped, true)
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
	assertRefsEqual(t, refNameTyped, repoRemote, repoLocal)
	assertRefsEqual(t, rslRefNameTyped, repoRemote, repoLocal)
	assertRefsEqual(t, policyRefNameTyped, repoRemote, repoLocal)

	// Verify remote doesn't have the second ref
	_, err = repoRemote.r.Reference(anotherRefNameTyped, true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)

	// Initialize second ref and add another commit to first ref
	localRef, err := repoLocal.r.Reference(refNameTyped, true)
	if err != nil {
		t.Fatal(err)
	}
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
	assertRefsEqual(t, refNameTyped, repoRemote, repoLocal)
	assertRefsEqual(t, anotherRefNameTyped, repoRemote, repoLocal)
	assertRefsEqual(t, rslRefNameTyped, repoRemote, repoLocal)
	assertRefsEqual(t, policyRefNameTyped, repoRemote, repoLocal)
}

func TestPull(t *testing.T) {
	remoteTmpDir := t.TempDir()

	remoteRepo := initializeTestRepositoryInDir(t, remoteTmpDir)

	refName := "refs/heads/main"
	anotherRefName := "refs/heads/feature"
	rslRefNameTyped := plumbing.ReferenceName(rsl.RSLRef)
	policyRefNameTyped := plumbing.ReferenceName(policy.PolicyRef)
	remoteName := "origin"

	recordChangesInTestRepository(t, remoteRepo, refName, []string{anotherRefName})

	// repository.Clone is an option but that needs a dir
	localR, err := gitinterface.CloneAndFetchToMemory(context.Background(), remoteTmpDir, "", []string{rsl.RSLRef, policy.PolicyRef}, false)
	if err != nil {
		t.Fatal(err)
	}
	localRepo := &Repository{r: localR}

	assertRefsEqual(t, rslRefNameTyped, remoteRepo, localRepo)
	assertRefsEqual(t, policyRefNameTyped, remoteRepo, localRepo)

	// Make remote changes
	_, err = gitinterface.Commit(remoteRepo.r, gitinterface.EmptyTree(), refName, "Another commit", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := remoteRepo.RecordRSLEntryForReference(refName, false); err != nil {
		t.Fatal(err)
	}

	err = localRepo.Pull(context.Background(), remoteName, refName)
	assert.Nil(t, err)

	assertRefsEqual(t, plumbing.ReferenceName(refName), remoteRepo, localRepo)

	// Assert second ref isn't in local
	_, err = localRepo.r.Reference(plumbing.ReferenceName(anotherRefName), true)
	assert.ErrorIs(t, err, plumbing.ErrReferenceNotFound)

	err = localRepo.Pull(context.Background(), remoteName, anotherRefName)
	assert.Nil(t, err)

	assertRefsEqual(t, plumbing.ReferenceName(anotherRefName), remoteRepo, localRepo)
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

func assertRefsEqual(t *testing.T, ref plumbing.ReferenceName, a, b *Repository) {
	t.Helper()

	aRef, err := a.r.Reference(ref, true)
	if err != nil {
		t.Fatal(err)
	}
	bRef, err := b.r.Reference(ref, true)
	if err != nil {
		t.Fatal(err)
	}

	assert.Equal(t, aRef.Hash(), bRef.Hash())
}

func initializeTestRepositoryInDir(t *testing.T, dir string) *Repository {
	t.Helper()

	remoteR, err := git.PlainInit(dir, true)
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
	_, err = gitinterface.WriteTree(remoteRepo.r, nil)
	if err != nil {
		t.Fatal(err)
	}

	return remoteRepo
}

func recordChangesInTestRepository(t *testing.T, repo *Repository, mainRef string, otherRefs []string) {
	t.Helper()

	commitID, err := gitinterface.Commit(repo.r, gitinterface.EmptyTree(), mainRef, "Initial commit", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.RecordRSLEntryForReference(mainRef, false); err != nil {
		t.Fatal(err)
	}
	if err := repo.r.Storer.SetReference(plumbing.NewSymbolicReference(plumbing.HEAD, plumbing.ReferenceName(mainRef))); err != nil {
		t.Fatal(err)
	}
	for _, ref := range otherRefs {
		if err := repo.r.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(ref), commitID)); err != nil {
			t.Fatal(err)
		}
		if err := repo.RecordRSLEntryForReference(ref, false); err != nil {
			t.Fatal(err)
		}
	}
}
