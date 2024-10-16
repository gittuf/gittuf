// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/stretchr/testify/assert"
)

func TestLoadRepository(t *testing.T) {
	tmpDir := t.TempDir()
	gitinterface.CreateTestGitRepository(t, tmpDir, false)

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(tmpDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(currentDir) //nolint:errcheck

	repository, err := LoadRepository()
	assert.Nil(t, err)
	assert.NotNil(t, repository.r)
}

func TestUnauthorizedKey(t *testing.T) {
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	rootSigner := setupSSHKeysForSigning(t, rootKeyBytes, rootPubKeyBytes)
	targetsSigner := setupSSHKeysForSigning(t, targetsKeyBytes, targetsPubKeyBytes)

	r := &Repository{r: repo}
	if err := r.InitializeRoot(testCtx, rootSigner, false); err != nil {
		t.Fatal(err)
	}

	t.Run("test add targets key", func(t *testing.T) {
		key := tufv01.NewKeyFromSSLibKey(targetsSigner.MetadataKey())

		err := r.AddTopLevelTargetsKey(testCtx, targetsSigner, key, false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})

	t.Run("test remove targets key", func(t *testing.T) {
		err := r.RemoveTopLevelTargetsKey(testCtx, targetsSigner, "some key ID", false)
		assert.ErrorIs(t, err, ErrUnauthorizedKey)
	})
}
