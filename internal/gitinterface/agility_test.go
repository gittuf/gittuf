package gitinterface

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/plumbing/filemode"
	"github.com/go-git/go-git/v5/plumbing/object"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestRecordHashEntry(t *testing.T) {
	t.Run("verify hash of empty blob", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		emptyBlobHash, err := WriteBlob(repo, nil)
		if err != nil {
			t.Fatal(err)
		}

		err = RecordHashEntry(repo, emptyBlobHash, SHA256HashAlg)
		assert.Nil(t, err)

		ref, err := repo.Reference(HashAgilityRef, true)
		if err != nil {
			t.Fatal(err)
		}
		mappingContents, err := ReadBlob(repo, ref.Hash())
		if err != nil {
			t.Fatal(err)
		}
		hashMapping := map[string]string{}
		if err := json.Unmarshal(mappingContents, &hashMapping); err != nil {
			t.Fatal(err)
		}

		sha256Hash, has := hashMapping[emptyBlobHash.String()]
		assert.True(t, has)
		assert.Equal(t, "473a0f4c3be8a93681a267e3b1e9a7dcda1185436fe141f7749120a303721813", sha256Hash)
	})

	t.Run("verify hash of empty tree", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		emptyTreeHash, err := WriteTree(repo, nil)
		if err != nil {
			t.Fatal(err)
		}

		err = RecordHashEntry(repo, emptyTreeHash, SHA256HashAlg)
		assert.Nil(t, err)

		ref, err := repo.Reference(HashAgilityRef, true)
		if err != nil {
			t.Fatal(err)
		}
		mappingContents, err := ReadBlob(repo, ref.Hash())
		if err != nil {
			t.Fatal(err)
		}
		hashMapping := map[string]string{}
		if err := json.Unmarshal(mappingContents, &hashMapping); err != nil {
			t.Fatal(err)
		}

		sha256Hash, has := hashMapping[emptyTreeHash.String()]
		assert.True(t, has)
		assert.Equal(t, "6ef19b41225c5369f1c104d45d8d85efa9b057b53b14b4b9b939dd74decc5321", sha256Hash)
	})

	t.Run("verify hash of tree with single blob", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		blobHash, err := WriteBlob(repo, []byte("Hello, world!\n"))
		if err != nil {
			t.Fatal(err)
		}

		// We do not record the blob's SHA-256 hash directly

		treeHash, err := WriteTree(repo, []object.TreeEntry{{
			Name: "README.md",
			Mode: filemode.Regular,
			Hash: blobHash,
		}})
		if err != nil {
			t.Fatal(err)
		}

		err = RecordHashEntry(repo, treeHash, SHA256HashAlg)
		assert.Nil(t, err)

		ref, err := repo.Reference(HashAgilityRef, true)
		if err != nil {
			t.Fatal(err)
		}
		mappingContents, err := ReadBlob(repo, ref.Hash())
		if err != nil {
			t.Fatal(err)
		}
		hashMapping := map[string]string{}
		if err := json.Unmarshal(mappingContents, &hashMapping); err != nil {
			t.Fatal(err)
		}

		sha256Hash, has := hashMapping[treeHash.String()]
		assert.True(t, has)
		assert.Equal(t, "63942a616285f13394900d0514b988b0b61d0b17bc37d74a71fc1051d3984094", sha256Hash)

		sha256Hash, has = hashMapping[blobHash.String()]
		assert.True(t, has)
		assert.Equal(t, "7506cbcf4c572be9e06a1fed35ac5b1df8b5a74d26c07f022648e5d95a9f6f2a", sha256Hash)
	})

	t.Run("verify hash of tree with subtree containing blob", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		blobHash, err := WriteBlob(repo, []byte("Hello, world!\n"))
		if err != nil {
			t.Fatal(err)
		}

		// We do not record the blob's SHA-256 hash directly

		subTreeHash, err := WriteTree(repo, []object.TreeEntry{{
			Name: "README.md",
			Mode: filemode.Regular,
			Hash: blobHash,
		}})
		if err != nil {
			t.Fatal(err)
		}

		treeHash, err := WriteTree(repo, []object.TreeEntry{{
			Name: "src",
			Mode: filemode.Dir,
			Hash: subTreeHash,
		}})
		if err != nil {
			t.Fatal(err)
		}

		err = RecordHashEntry(repo, treeHash, SHA256HashAlg)
		assert.Nil(t, err)

		ref, err := repo.Reference(HashAgilityRef, true)
		if err != nil {
			t.Fatal(err)
		}
		mappingContents, err := ReadBlob(repo, ref.Hash())
		if err != nil {
			t.Fatal(err)
		}
		hashMapping := map[string]string{}
		if err := json.Unmarshal(mappingContents, &hashMapping); err != nil {
			t.Fatal(err)
		}

		sha256Hash, has := hashMapping[treeHash.String()]
		assert.True(t, has)
		assert.Equal(t, "617afa726069a80f7855a13401c7eb2010aba0934944c6e3a631c948e75e76eb", sha256Hash)

		sha256Hash, has = hashMapping[subTreeHash.String()]
		assert.True(t, has)
		assert.Equal(t, "63942a616285f13394900d0514b988b0b61d0b17bc37d74a71fc1051d3984094", sha256Hash)

		sha256Hash, has = hashMapping[blobHash.String()]
		assert.True(t, has)
		assert.Equal(t, "7506cbcf4c572be9e06a1fed35ac5b1df8b5a74d26c07f022648e5d95a9f6f2a", sha256Hash)
	})

	t.Run("verify unsigned commit hashes against SHA-256 repo", func(t *testing.T) {
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		fileName := "README.md"
		fileContents := "Hello, world!\n"
		commitMessage := "Test commit\n"

		cmd := exec.Command("git", "init", "--object-format=sha256")
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("git", "config", "--local", "user.name", fmt.Sprintf("'%s'", testName))
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("git", "config", "--local", "user.email", fmt.Sprintf("'%s'", testEmail))
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("git", "config", "--local", "commit.gpgsign", "false")
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(fileName, []byte(fileContents), 0644); err != nil {
			t.Fatal(err)
		}

		cmd = exec.Command("git", "add", fileName)
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("git", "commit", "-m", commitMessage)
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_AUTHOR_DATE=%s", testClock.Now().Format(time.RFC3339)))
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_DATE=%s", testClock.Now().Format(time.RFC3339)))
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}

		cmd = exec.Command("git", "rev-parse", "HEAD")
		stdOut, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		expectedFirstCommitHash := strings.TrimSpace(string(stdOut))

		cmd = exec.Command("git", "commit", "-m", commitMessage, "--allow-empty")
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_AUTHOR_DATE=%s", testClock.Now().Format(time.RFC3339)))
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_DATE=%s", testClock.Now().Format(time.RFC3339)))
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}

		cmd = exec.Command("git", "rev-parse", "HEAD")
		stdOut, err = cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		expectedSecondCommitHash := strings.TrimSpace(string(stdOut))

		// Early check against constants before checking the in-memory SHA-1
		// repo's result
		assert.Equal(t, "e5626d7bcaaaf8139c9d10844ecd17febe766140b9a53309244c7cf800907773", expectedFirstCommitHash)
		assert.Equal(t, "acefa317648648a920b76cd982635c8e99060176811cff8b7ef54285e4d853be", expectedSecondCommitHash)

		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.SetConfig(testGitConfig); err != nil {
			t.Fatal(err)
		}

		clock = testClock

		blobHash, err := WriteBlob(repo, []byte(fileContents))
		if err != nil {
			t.Fatal(err)
		}

		treeHash, err := WriteTree(repo, []object.TreeEntry{{
			Name: fileName,
			Mode: filemode.Regular,
			Hash: blobHash,
		}})
		if err != nil {
			t.Fatal(err)
		}

		refName := "refs/heads/main"
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}
		commitID, err := Commit(repo, treeHash, refName, commitMessage, false)
		if err != nil {
			t.Fatal(err)
		}

		err = RecordHashEntry(repo, commitID, SHA256HashAlg)
		assert.Nil(t, err)

		ref, err := repo.Reference(HashAgilityRef, true)
		if err != nil {
			t.Fatal(err)
		}
		mappingContents, err := ReadBlob(repo, ref.Hash())
		if err != nil {
			t.Fatal(err)
		}
		hashMapping := map[string]string{}
		if err := json.Unmarshal(mappingContents, &hashMapping); err != nil {
			t.Fatal(err)
		}

		firstCommitHash, has := hashMapping[commitID.String()]
		assert.True(t, has)
		assert.Equal(t, expectedFirstCommitHash, firstCommitHash)

		commitID, err = Commit(repo, treeHash, refName, commitMessage, false)
		if err != nil {
			t.Fatal(err)
		}

		err = RecordHashEntry(repo, commitID, SHA256HashAlg)
		assert.Nil(t, err)

		ref, err = repo.Reference(HashAgilityRef, true)
		if err != nil {
			t.Fatal(err)
		}
		mappingContents, err = ReadBlob(repo, ref.Hash())
		if err != nil {
			t.Fatal(err)
		}
		hashMapping = map[string]string{}
		if err := json.Unmarshal(mappingContents, &hashMapping); err != nil {
			t.Fatal(err)
		}

		secondCommitHash, has := hashMapping[commitID.String()]
		assert.True(t, has)
		assert.Equal(t, expectedSecondCommitHash, secondCommitHash)
	})

	t.Run("verify unsigned tag hash against SHA-256 repo", func(t *testing.T) {
		currentDir, err := os.Getwd()
		if err != nil {
			t.Fatal(err)
		}
		tmpDir, err := os.MkdirTemp("", "gittuf")
		if err != nil {
			t.Fatal(err)
		}
		defer os.Chdir(currentDir) //nolint:errcheck
		defer os.RemoveAll(tmpDir) //nolint:errcheck

		if err := os.Chdir(tmpDir); err != nil {
			t.Fatal(err)
		}

		fileName := "README.md"
		fileContents := "Hello, world!\n"
		commitMessage := "Test commit\n"
		tagName := "v0.1.0"
		tagMessage := "First release\n"

		cmd := exec.Command("git", "init", "--object-format=sha256")
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("git", "config", "--local", "user.name", fmt.Sprintf("'%s'", testName))
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("git", "config", "--local", "user.email", fmt.Sprintf("'%s'", testEmail))
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("git", "config", "--local", "commit.gpgsign", "false")
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}

		if err := os.WriteFile(fileName, []byte(fileContents), 0644); err != nil {
			t.Fatal(err)
		}

		cmd = exec.Command("git", "add", fileName)
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}
		cmd = exec.Command("git", "commit", "-m", commitMessage)
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_AUTHOR_DATE=%s", testClock.Now().Format(time.RFC3339)))
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_DATE=%s", testClock.Now().Format(time.RFC3339)))
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}

		cmd = exec.Command("git", "tag", "-m", tagMessage, tagName)
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_AUTHOR_DATE=%s", testClock.Now().Format(time.RFC3339)))
		cmd.Env = append(cmd.Env, fmt.Sprintf("GIT_COMMITTER_DATE=%s", testClock.Now().Format(time.RFC3339)))
		if err := cmd.Run(); err != nil {
			t.Fatal(err)
		}

		cmd = exec.Command("git", "rev-parse", tagName)
		stdOut, err := cmd.Output()
		if err != nil {
			t.Fatal(err)
		}
		expectedTagHash := strings.TrimSpace(string(stdOut))

		// Early check against constant before checking the in-memory SHA-1
		// repo's result
		assert.Equal(t, "163515dcf78aebadf7a0da7b9bf61038093d95f3a63b30a9d5c18c90c908e9d0", expectedTagHash)

		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := repo.SetConfig(testGitConfig); err != nil {
			t.Fatal(err)
		}

		clock = testClock

		blobHash, err := WriteBlob(repo, []byte(fileContents))
		if err != nil {
			t.Fatal(err)
		}

		treeHash, err := WriteTree(repo, []object.TreeEntry{{
			Name: fileName,
			Mode: filemode.Regular,
			Hash: blobHash,
		}})
		if err != nil {
			t.Fatal(err)
		}

		refName := "refs/heads/main"
		if err := repo.Storer.SetReference(plumbing.NewHashReference(plumbing.ReferenceName(refName), plumbing.ZeroHash)); err != nil {
			t.Fatal(err)
		}
		commitID, err := Commit(repo, treeHash, refName, commitMessage, false)
		if err != nil {
			t.Fatal(err)
		}

		tagHash, err := Tag(repo, commitID, tagName, tagMessage, false)
		if err != nil {
			t.Fatal(err)
		}

		err = RecordHashEntry(repo, tagHash, SHA256HashAlg)
		assert.Nil(t, err)

		ref, err := repo.Reference(HashAgilityRef, true)
		if err != nil {
			t.Fatal(err)
		}
		mappingContents, err := ReadBlob(repo, ref.Hash())
		if err != nil {
			t.Fatal(err)
		}
		hashMapping := map[string]string{}
		if err := json.Unmarshal(mappingContents, &hashMapping); err != nil {
			t.Fatal(err)
		}

		tagSHA256Hash, has := hashMapping[tagHash.String()]
		assert.True(t, has)
		assert.Equal(t, expectedTagHash, tagSHA256Hash)
	})
}
