// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"context"
	"os"
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/go-git/go-git/v5"
	"github.com/stretchr/testify/assert"
)

func TestAddAndRemoveReferenceAuthorization(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	testDir := t.TempDir()

	currentDir, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Chdir(testDir); err != nil {
		t.Fatal(err)
	}
	defer os.Chdir(currentDir) //nolint:errcheck

	r, err := git.PlainInit(testDir, false)
	if err != nil {
		t.Fatal(err)
	}
	repo := &Repository{r: r}
	if err := repo.InitializeNamespaces(); err != nil {
		t.Fatal(err)
	}

	// Create main branch as the target branch with a Git commit
	targetRef := "main"
	absTargetRef := "refs/heads/main"
	// Add a single commit
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, r, absTargetRef, 1, gpgKeyBytes)
	fromCommitID := commitIDs[0].String()
	if err := repo.RecordRSLEntryForReference(targetRef, false); err != nil {
		t.Fatal(err)
	}

	// Create feature branch with two Git commits
	featureRef := "feature"
	absFeatureRef := "refs/heads/feature"
	// Add two commits
	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, r, absFeatureRef, 2, gpgKeyBytes)
	featureCommitID := commitIDs[1].String()
	if err := repo.RecordRSLEntryForReference(featureRef, false); err != nil {
		t.Fatal(err)
	}

	targetTreeID, err := gitinterface.GetMergeTree(r, fromCommitID, featureCommitID)
	if err != nil {
		t.Fatal(err)
	}

	// Create signers
	firstSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(rootKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}
	firstKeyID, err := firstSigner.KeyID()
	if err != nil {
		t.Fatal(err)
	}
	secondSigner, err := signerverifier.NewSignerVerifierFromSecureSystemsLibFormat(targetsKeyBytes) //nolint:staticcheck
	if err != nil {
		t.Fatal(err)
	}
	secondKeyID, err := secondSigner.KeyID()
	if err != nil {
		t.Fatal(err)
	}

	// First authorization attestation signature
	err = repo.AddReferenceAuthorization(context.Background(), firstSigner, absTargetRef, absFeatureRef, false)
	assert.Nil(t, err)

	allAttestations, err := attestations.LoadCurrentAttestations(r)
	if err != nil {
		t.Fatal(err)
	}

	env, err := allAttestations.GetReferenceAuthorizationFor(r, absTargetRef, fromCommitID, targetTreeID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, env.Signatures, 1)
	assert.Equal(t, firstKeyID, env.Signatures[0].KeyID)

	// Second authorization attestation signature
	err = repo.AddReferenceAuthorization(context.Background(), secondSigner, absTargetRef, absFeatureRef, false)
	assert.Nil(t, err)

	allAttestations, err = attestations.LoadCurrentAttestations(r)
	if err != nil {
		t.Fatal(err)
	}

	env, err = allAttestations.GetReferenceAuthorizationFor(r, absTargetRef, fromCommitID, targetTreeID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, env.Signatures, 2)
	assert.Equal(t, firstKeyID, env.Signatures[0].KeyID)
	assert.Equal(t, secondKeyID, env.Signatures[1].KeyID)

	// Remove second authorization attestation signature
	err = repo.RemoveReferenceAuthorization(context.Background(), secondSigner, absTargetRef, fromCommitID, targetTreeID, false)
	assert.Nil(t, err)

	allAttestations, err = attestations.LoadCurrentAttestations(r)
	if err != nil {
		t.Fatal(err)
	}

	env, err = allAttestations.GetReferenceAuthorizationFor(r, absTargetRef, fromCommitID, targetTreeID)
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, env.Signatures, 1)
	assert.Equal(t, firstKeyID, env.Signatures[0].KeyID)
}
