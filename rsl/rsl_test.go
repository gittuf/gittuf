package rsl

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/adityasaky/gittuf/internal/common"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/stretchr/testify/assert"
)

func TestInitializeNamespace(t *testing.T) {
	testDir, err := common.CreateTestRepository()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(); err != nil {
		t.Error(err)
	}

	refContents, err := os.ReadFile(filepath.Join(".git", RSLRef))
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, plumbing.ZeroHash, plumbing.NewHash(string(refContents)))
}

func TestAddEntry(t *testing.T) {
	testDir, err := common.CreateTestRepository()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(); err != nil {
		t.Error(err)
	}

	if err := AddEntry("main", plumbing.ZeroHash, false); err != nil {
		t.Error(err)
	}

	refContents, err := os.ReadFile(filepath.Join(".git", RSLRef))
	if err != nil {
		t.Error(err)
	}
	refHash := plumbing.NewHash(string(refContents))
	assert.NotEqual(t, plumbing.ZeroHash, refHash)

	repo, err := common.GetRepositoryHandler()
	if err != nil {
		t.Error(err)
	}

	commitObj, err := repo.CommitObject(refHash)
	if err != nil {
		t.Error(err)
	}

	expectedMessage := fmt.Sprintf("%s: %s", "main", plumbing.ZeroHash.String())
	assert.Equal(t, expectedMessage, commitObj.Message)

	assert.Empty(t, commitObj.ParentHashes)

	if err := AddEntry("main", plumbing.NewHash("abcdef1234567890"), false); err != nil {
		t.Error(err)
	}
	refContents, err = os.ReadFile(filepath.Join(".git", RSLRef))
	if err != nil {
		t.Error(err)
	}
	newRefHash := plumbing.NewHash(string(refContents))

	commitObj, err = repo.CommitObject(newRefHash)
	if err != nil {
		t.Error(err)
	}

	expectedMessage = fmt.Sprintf("%s: %s", "main", plumbing.NewHash("abcdef1234567890"))
	assert.Equal(t, expectedMessage, commitObj.Message)

	assert.Contains(t, commitObj.ParentHashes, refHash)
}

func TestGetLatestEntry(t *testing.T) {
	testDir, err := common.CreateTestRepository()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}

	if err := InitializeNamespace(); err != nil {
		t.Error(err)
	}

	if err := AddEntry("main", plumbing.ZeroHash, false); err != nil {
		t.Error(err)
	}

	if ref, commitID, err := GetLatestEntry(); err != nil {
		t.Error(err)
	} else {
		assert.Equal(t, "main", ref)
		assert.Equal(t, plumbing.ZeroHash, commitID)
	}

	if err := AddEntry("feature",
		plumbing.NewHash("abcdef1234567890"), false); err != nil {
		t.Error(err)
	}
	if ref, commitID, err := GetLatestEntry(); err != nil {
		t.Error(err)
	} else {
		assert.NotEqual(t, "main", ref)
		assert.NotEqual(t, plumbing.ZeroHash, commitID)
	}
}
