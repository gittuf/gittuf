// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"encoding/json"
	"path"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/stretchr/testify/assert"
)

func TestLoadCurrentAttestations(t *testing.T) {
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	testAttestation, err := NewReferenceAuthorization(testRef, testID, testID)
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
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		attestations, err := LoadCurrentAttestations(repo)
		assert.Nil(t, err)
		assert.Empty(t, attestations.referenceAuthorizations)
	})

	t.Run("with RSL entry and with an attestation", func(t *testing.T) {
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		blobID, err := repo.WriteBlob(testEnvBytes)
		if err != nil {
			t.Fatal(err)
		}

		authorizations := map[string]gitinterface.Hash{ReferenceAuthorizationPath(testRef, testID, testID): blobID}

		attestations := &Attestations{referenceAuthorizations: authorizations}
		if err := attestations.Commit(repo, "Test commit", false); err != nil {
			t.Fatal(err)
		}

		attestations, err = LoadCurrentAttestations(repo)
		assert.Nil(t, err)
		assert.Equal(t, authorizations, attestations.referenceAuthorizations)
	})
}

func TestLoadAttestationsForEntry(t *testing.T) {
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	testAttestation, err := NewReferenceAuthorization(testRef, testID, testID)
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
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		authorizations := map[string]gitinterface.Hash{}

		attestations := &Attestations{referenceAuthorizations: authorizations}
		if err := attestations.Commit(repo, "Test commit", false); err != nil {
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
		tempDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

		blobID, err := repo.WriteBlob(testEnvBytes)
		if err != nil {
			t.Fatal(err)
		}

		authorizations := map[string]gitinterface.Hash{ReferenceAuthorizationPath(testRef, testID, testID): blobID}

		attestations := &Attestations{referenceAuthorizations: authorizations}
		if err := attestations.Commit(repo, "Test commit", false); err != nil {
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
	testRef := "refs/heads/main"
	testID := gitinterface.ZeroHash.String()
	testAttestation, err := NewReferenceAuthorization(testRef, testID, testID)
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

	treeBuilder := gitinterface.NewReplacementTreeBuilder(repo)
	expectedTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(map[string]gitinterface.Hash{path.Join(referenceAuthorizationsTreeEntryName, ReferenceAuthorizationPath(testRef, testID, testID)): blobID})
	if err != nil {
		t.Fatal(err)
	}

	if err := attestations.Commit(repo, "Test commit", false); err != nil {
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
