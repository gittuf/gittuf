// SPDX-License-Identifier: Apache-2.0

package repository

import (
	"testing"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/common"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/stretchr/testify/assert"
)

func TestAddAndRemoveReferenceAuthorization(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	testDir := t.TempDir()
	r := gitinterface.CreateTestGitRepository(t, testDir, false)

	repo := &Repository{r: r}

	targetRef := "main"
	absTargetRef := "refs/heads/main"
	featureRef := "feature"
	absFeatureRef := "refs/heads/feature"

	// Create common base for main and feature branches
	treeBuilder := gitinterface.NewTreeBuilder(repo.r)
	emptyTreeID, err := treeBuilder.WriteRootTreeFromBlobIDs(nil)
	if err != nil {
		t.Fatal(err)
	}
	initialCommitID, err := repo.r.Commit(emptyTreeID, absTargetRef, "Initial commit\n", false)
	if err != nil {
		t.Fatal(err)
	}
	if err := repo.r.SetReference(absFeatureRef, initialCommitID); err != nil {
		t.Fatal(err)
	}

	// Create main branch as the target branch with a Git commit
	// Add a single commit
	commitIDs := common.AddNTestCommitsToSpecifiedRef(t, r, absTargetRef, 1, gpgKeyBytes)
	fromCommitID := commitIDs[0]
	if err := repo.RecordRSLEntryForReference(targetRef, false); err != nil {
		t.Fatal(err)
	}

	// Create feature branch with two Git commits
	// Add two commits
	commitIDs = common.AddNTestCommitsToSpecifiedRef(t, r, absFeatureRef, 2, gpgKeyBytes)
	featureCommitID := commitIDs[1]
	if err := repo.RecordRSLEntryForReference(featureRef, false); err != nil {
		t.Fatal(err)
	}

	targetTreeID, err := r.GetMergeTree(fromCommitID, featureCommitID)
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
	err = repo.AddReferenceAuthorization(testCtx, firstSigner, absTargetRef, absFeatureRef, false)
	assert.Nil(t, err)

	allAttestations, err := attestations.LoadCurrentAttestations(r)
	if err != nil {
		t.Fatal(err)
	}

	env, err := allAttestations.GetReferenceAuthorizationFor(r, absTargetRef, fromCommitID.String(), targetTreeID.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, env.Signatures, 1)
	assert.Equal(t, firstKeyID, env.Signatures[0].KeyID)

	// Second authorization attestation signature
	err = repo.AddReferenceAuthorization(testCtx, secondSigner, absTargetRef, absFeatureRef, false)
	assert.Nil(t, err)

	allAttestations, err = attestations.LoadCurrentAttestations(r)
	if err != nil {
		t.Fatal(err)
	}

	env, err = allAttestations.GetReferenceAuthorizationFor(r, absTargetRef, fromCommitID.String(), targetTreeID.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, env.Signatures, 2)
	assert.Equal(t, firstKeyID, env.Signatures[0].KeyID)
	assert.Equal(t, secondKeyID, env.Signatures[1].KeyID)

	// Remove second authorization attestation signature
	err = repo.RemoveReferenceAuthorization(testCtx, secondSigner, absTargetRef, fromCommitID.String(), targetTreeID.String(), false)
	assert.Nil(t, err)

	allAttestations, err = attestations.LoadCurrentAttestations(r)
	if err != nil {
		t.Fatal(err)
	}

	env, err = allAttestations.GetReferenceAuthorizationFor(r, absTargetRef, fromCommitID.String(), targetTreeID.String())
	if err != nil {
		t.Fatal(err)
	}
	assert.Len(t, env.Signatures, 1)
	assert.Equal(t, firstKeyID, env.Signatures[0].KeyID)
}
