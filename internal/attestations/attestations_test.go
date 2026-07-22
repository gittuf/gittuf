// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"
	"errors"
	"path"
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/gittuf/gittuf/pkg/rsl"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadCurrentAttestations(t *testing.T) {
	t.Parallel()
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	testAttestation, err := NewReferenceAuthorizationForCommit(testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	testEnv, err := dsse.CreateEnvelope(testAttestation)
	if err != nil {
		t.Fatal(err)
	}
	testEnvBytes, err := json.Marshal(testEnv)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("no RSL entry", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		attestations, err := LoadCurrentAttestations(repo)
		assert.Nil(t, err)
		assert.Empty(t, attestations.referenceAuthorizations)
	})

	t.Run("with RSL entry and with an attestation", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		blobID, err := repo.WriteBlob(testEnvBytes)
		if err != nil {
			t.Fatal(err)
		}

		authorizations := map[string]gitinterface.Hash{ReferenceAuthorizationPath(testRef, testID, testID): blobID}

		attestations := &Attestations{referenceAuthorizations: authorizations}
		if err := attestations.Commit(repo, "Test commit", true, false); err != nil {
			t.Fatal(err)
		}

		attestations, err = LoadCurrentAttestations(repo)
		assert.Nil(t, err)
		assert.Equal(t, authorizations, attestations.referenceAuthorizations)
	})
}

func TestLoadAttestationsForEntry(t *testing.T) {
	t.Parallel()
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	testAttestation, err := NewReferenceAuthorizationForCommit(testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	testEnv, err := dsse.CreateEnvelope(testAttestation)
	if err != nil {
		t.Fatal(err)
	}
	testEnvBytes, err := json.Marshal(testEnv)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("with RSL entry and no an attestation", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		authorizations := map[string]gitinterface.Hash{}

		attestations := &Attestations{referenceAuthorizations: authorizations}
		if err := attestations.Commit(repo, "Test commit", true, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		attestations, err = LoadAttestationsForEntry(repo, entry.(*rsl.ReferenceEntry))
		assert.Nil(t, err)
		assert.Empty(t, attestations.referenceAuthorizations)
	})

	t.Run("with RSL entry and with an attestation", func(t *testing.T) {
		t.Parallel()

		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		blobID, err := repo.WriteBlob(testEnvBytes)
		if err != nil {
			t.Fatal(err)
		}

		authorizations := map[string]gitinterface.Hash{ReferenceAuthorizationPath(testRef, testID, testID): blobID}

		attestations := &Attestations{referenceAuthorizations: authorizations}
		if err := attestations.Commit(repo, "Test commit", true, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		attestations, err = LoadAttestationsForEntry(repo, entry.(*rsl.ReferenceEntry))
		assert.Nil(t, err)
		assert.Equal(t, authorizations, attestations.referenceAuthorizations)
	})
}

func TestAttestationsCommit(t *testing.T) {
	t.Parallel()
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	testAttestation, err := NewReferenceAuthorizationForCommit(testRef, testID, testID)
	if err != nil {
		t.Fatal(err)
	}
	testEnv, err := dsse.CreateEnvelope(testAttestation)
	if err != nil {
		t.Fatal(err)
	}
	testEnvBytes, err := json.Marshal(testEnv)
	if err != nil {
		t.Fatal(err)
	}

	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	blobID, err := repo.WriteBlob(testEnvBytes)
	if err != nil {
		t.Fatal(err)
	}

	authorizations := map[string]gitinterface.Hash{ReferenceAuthorizationPath(testRef, testID, testID): blobID}
	attestations := &Attestations{referenceAuthorizations: authorizations}

	treeBuilder := gitinterface.NewTreeBuilder(repo)
	expectedTreeID, err := treeBuilder.WriteTreeFromEntries([]gitinterface.TreeEntry{gitinterface.NewEntryBlob(path.Join(referenceAuthorizationsTreeEntryName, ReferenceAuthorizationPath(testRef, testID, testID)), blobID)})
	if err != nil {
		t.Fatal(err)
	}

	if err := attestations.Commit(repo, "Test commit", true, false); err != nil {
		t.Error(err)
	}

	currentTip, err := repo.GetReference(Ref)
	if err != nil {
		t.Fatal(err)
	}
	currentTreeID, err := repo.GetCommitTreeID(currentTip)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, expectedTreeID, currentTreeID)

	attestations, err = LoadCurrentAttestations(repo)
	assert.Nil(t, err)
	assert.Equal(t, attestations.referenceAuthorizations, authorizations)
}

// overrideStorer wraps a real Storer, injecting failures for the methods the
// tests need to see fail while delegating everything else.
type overrideStorer struct {
	gitstore.Storer

	emptyTreeErr         error
	getReferenceErr      error
	getCommitTreeIDErr   error
	getAllFilesInTreeErr error
	writeTreeErr         error
	commitErr            error
}

func (o *overrideStorer) EmptyTree() (gitinterface.Hash, error) {
	if o.emptyTreeErr != nil {
		return nil, o.emptyTreeErr
	}
	return o.Storer.EmptyTree()
}

func (o *overrideStorer) GetReference(refName string) (gitinterface.Hash, error) {
	if o.getReferenceErr != nil {
		return nil, o.getReferenceErr
	}
	return o.Storer.GetReference(refName)
}

func (o *overrideStorer) GetCommitTreeID(commitID gitinterface.Hash) (gitinterface.Hash, error) {
	if o.getCommitTreeIDErr != nil {
		return nil, o.getCommitTreeIDErr
	}
	return o.Storer.GetCommitTreeID(commitID)
}

func (o *overrideStorer) GetAllFilesInTree(treeID gitinterface.Hash) (map[string]gitinterface.Hash, error) {
	if o.getAllFilesInTreeErr != nil {
		return nil, o.getAllFilesInTreeErr
	}
	return o.Storer.GetAllFilesInTree(treeID)
}

func (o *overrideStorer) WriteTree(blobs, subtrees map[string]gitinterface.Hash) (gitinterface.Hash, error) {
	if o.writeTreeErr != nil {
		return nil, o.writeTreeErr
	}
	return o.Storer.WriteTree(blobs, subtrees)
}

func (o *overrideStorer) Commit(treeID gitinterface.Hash, targetRef, message string, sign bool) (gitinterface.Hash, error) {
	if o.commitErr != nil {
		return nil, o.commitErr
	}
	return o.Storer.Commit(treeID, targetRef, message, sign)
}

func TestAttestationsCommitStorerErrors(t *testing.T) {
	t.Parallel()

	t.Run("write tree error", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		injected := errors.New("write tree failure")
		attestations := &Attestations{}

		err := attestations.Commit(&overrideStorer{Storer: repo, writeTreeErr: injected}, "Test commit", false, false)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("get reference error", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		injected := errors.New("get reference failure")
		attestations := &Attestations{}

		err := attestations.Commit(&overrideStorer{Storer: repo, getReferenceErr: injected}, "Test commit", false, false)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("commit error", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		injected := errors.New("commit failure")
		attestations := &Attestations{}

		err := attestations.Commit(&overrideStorer{Storer: repo, commitErr: injected}, "Test commit", false, false)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("RSL entry error without prior commit", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		// The injected EmptyTree error only affects the RSL entry commit.
		// The attestations commit itself does not use the empty tree.
		injected := errors.New("empty tree failure")
		attestations := &Attestations{}

		err := attestations.Commit(&overrideStorer{Storer: repo, emptyTreeErr: injected}, "Test commit", true, false)
		assert.ErrorIs(t, err, injected)
	})

	t.Run("RSL entry error resets to prior commit", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		attestations := &Attestations{}
		require.Nil(t, attestations.Commit(repo, "First commit", false, false))
		priorCommitID, err := repo.GetReference(Ref)
		require.Nil(t, err)

		injected := errors.New("empty tree failure")
		err = attestations.Commit(&overrideStorer{Storer: repo, emptyTreeErr: injected}, "Second commit", true, false)
		assert.ErrorIs(t, err, injected)

		currentCommitID, err := repo.GetReference(Ref)
		require.Nil(t, err)
		assert.Equal(t, priorCommitID, currentCommitID)
	})
}

func TestAttestationsCommitAndLoadAllAttestationTypes(t *testing.T) {
	t.Parallel()

	repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)

	blobID, err := repo.WriteBlob([]byte("test attestation"))
	require.Nil(t, err)

	attestations := &Attestations{
		referenceAuthorizations:        map[string]gitinterface.Hash{"refs/heads/main/from-to": blobID},
		githubPullRequestAttestations:  map[string]gitinterface.Hash{"refs/heads/main/commit": blobID},
		codeReviewApprovalAttestations: map[string]gitinterface.Hash{"refs/heads/main/from-to/github": blobID},
		codeReviewApprovalIndex:        map[string]string{"github-1": "refs/heads/main/from-to/github"},
	}

	// The empty commit message exercises the default message path.
	err = attestations.Commit(repo, "", true, false)
	require.Nil(t, err)

	loaded, err := LoadCurrentAttestations(repo)
	require.Nil(t, err)

	assert.Equal(t, attestations.referenceAuthorizations, loaded.referenceAuthorizations)
	assert.Equal(t, attestations.githubPullRequestAttestations, loaded.githubPullRequestAttestations)
	assert.Equal(t, attestations.codeReviewApprovalAttestations, loaded.codeReviewApprovalAttestations)
	assert.Equal(t, attestations.codeReviewApprovalIndex, loaded.codeReviewApprovalIndex)
	assert.Contains(t, loaded.codeReviewApprovalAttestations, codeReviewApprovalIndexTreeEntryName)
}

func TestLoadCurrentAttestationsStorerError(t *testing.T) {
	t.Parallel()

	repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	injected := errors.New("get reference failure")

	_, err := LoadCurrentAttestations(&overrideStorer{Storer: repo, getReferenceErr: injected})
	assert.ErrorIs(t, err, injected)
}

func TestLoadAttestationsForEntryStorerErrors(t *testing.T) {
	t.Parallel()

	t.Run("entry for wrong ref", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)

		_, err := LoadAttestationsForEntry(repo, rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash))
		assert.ErrorIs(t, err, rsl.ErrRSLEntryDoesNotMatchRef)
	})

	t.Run("get commit tree ID error", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		injected := errors.New("get commit tree ID failure")

		_, err := LoadAttestationsForEntry(&overrideStorer{Storer: repo, getCommitTreeIDErr: injected}, rsl.NewReferenceEntry(Ref, gitinterface.ZeroHash))
		assert.ErrorIs(t, err, injected)
	})

	t.Run("get all files in tree error", func(t *testing.T) {
		t.Parallel()

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		attestations := &Attestations{}
		require.Nil(t, attestations.Commit(repo, "Test commit", true, false))
		entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo, rsl.ForReference(Ref))
		require.Nil(t, err)

		injected := errors.New("get all files in tree failure")
		_, err = LoadAttestationsForEntry(&overrideStorer{Storer: repo, getAllFilesInTreeErr: injected}, entry)
		assert.ErrorIs(t, err, injected)
	})
}
