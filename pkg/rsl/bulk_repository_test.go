// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package rsl

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBulkReferenceEntryCommit(t *testing.T) {
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	updates := []ReferenceUpdate{
		{RefName: "refs/heads/main", TargetID: gitinterface.ZeroHash},
		{RefName: "refs/heads/feature", TargetID: gitinterface.ZeroHash},
	}
	if err := NewBulkReferenceEntry(updates).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	latest, err := GetLatestEntry(repo)
	require.NoError(t, err)

	bulk, isBulk := latest.(*BulkReferenceEntry)
	require.True(t, isBulk)
	assert.Equal(t, uint64(2), bulk.Number)
	assert.Equal(t, updates, bulk.Updates)

	views := bulk.ReferenceEntries()
	require.Len(t, views, 2)
	for i, view := range views {
		assert.Equal(t, bulk.ID, view.ID)
		assert.Equal(t, bulk.Number, view.Number)
		assert.Equal(t, updates[i].RefName, view.RefName)
		assert.True(t, view.FromBulkEntry())
	}

	assert.ErrorIs(t, views[0].Commit(repo, false), ErrCannotCommitView)
	assert.ErrorIs(t, views[0].CommitUsingSpecificKey(repo, nil), ErrCannotCommitView)
	assert.ErrorIs(t, views[0].CommitWithoutNumber(repo), ErrCannotCommitView)

	parent, err := GetParentForEntry(repo, views[1])
	require.NoError(t, err)
	assert.Equal(t, uint64(1), parent.GetNumber())

	commitMessage, err := repo.GetCommitMessage(bulk.ID)
	require.NoError(t, err)
	expected := fmt.Sprintf("%s\n\nrefs/heads/main: %s\nrefs/heads/feature: %s\n\n%s: %d", BulkReferenceEntryHeader, gitinterface.ZeroHash.String(), gitinterface.ZeroHash.String(), NumberKey, 2)
	assert.Equal(t, expected, commitMessage)
}

func TestBulkReferenceEntryCommitRejectsInvalidUpdates(t *testing.T) {
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	err := NewBulkReferenceEntry([]ReferenceUpdate{
		{RefName: "refs/heads/main", TargetID: gitinterface.ZeroHash},
		{RefName: "refs/gittuf/policy", TargetID: gitinterface.ZeroHash},
	}).Commit(repo, false)
	assert.ErrorIs(t, err, ErrGittufReferenceInBulkEntry)

	_, err = GetLatestEntry(repo)
	assert.ErrorIs(t, err, ErrRSLEntryNotFound)
}

func TestGetReferenceUpdaterEntryForRef(t *testing.T) {
	tempDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tempDir, false)

	if err := NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	single, err := GetLatestEntry(repo)
	require.NoError(t, err)

	if err := NewBulkReferenceEntry([]ReferenceUpdate{
		{RefName: "refs/heads/main", TargetID: gitinterface.ZeroHash},
		{RefName: "refs/heads/feature", TargetID: gitinterface.ZeroHash},
	}).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	bulk, err := GetLatestEntry(repo)
	require.NoError(t, err)

	entry, err := GetReferenceUpdaterEntryForRef(repo, single.GetID(), "refs/heads/main")
	require.NoError(t, err)
	assert.Equal(t, single, entry)

	_, err = GetReferenceUpdaterEntryForRef(repo, single.GetID(), "refs/heads/feature")
	assert.ErrorIs(t, err, ErrRSLEntryDoesNotMatchRef)

	entry, err = GetReferenceUpdaterEntryForRef(repo, bulk.GetID(), "refs/heads/feature")
	require.NoError(t, err)
	assert.Equal(t, "refs/heads/feature", entry.GetRefName())
	assert.Equal(t, bulk.GetID(), entry.GetID())
	assert.True(t, entry.(*ReferenceEntry).FromBulkEntry())

	_, err = GetReferenceUpdaterEntryForRef(repo, bulk.GetID(), "refs/heads/other")
	assert.ErrorIs(t, err, ErrRSLEntryDoesNotMatchRef)

	if err := NewAnnotationEntry([]githash.Hash{bulk.GetID()}, false, annotationMessage).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	annotation, err := GetLatestEntry(repo)
	require.NoError(t, err)

	_, err = GetReferenceUpdaterEntryForRef(repo, annotation.GetID(), "refs/heads/main")
	assert.ErrorIs(t, err, ErrRSLEntryDoesNotMatchRef)

	if err := NewPropagationEntry("refs/heads/downstream", gitinterface.ZeroHash, "https://example.com/upstream", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}
	propagation, err := GetLatestEntry(repo)
	require.NoError(t, err)

	entry, err = GetReferenceUpdaterEntryForRef(repo, propagation.GetID(), "refs/heads/downstream")
	require.NoError(t, err)
	assert.Equal(t, propagation, entry)

	_, err = GetReferenceUpdaterEntryForRef(repo, propagation.GetID(), "refs/heads/main")
	assert.ErrorIs(t, err, ErrRSLEntryDoesNotMatchRef)

	unknownID, err := NewHash("abcdef12345678900987654321fedcbaabcdef12")
	require.NoError(t, err)

	_, err = GetReferenceUpdaterEntryForRef(repo, unknownID, "refs/heads/main")
	assert.ErrorIs(t, err, ErrRSLEntryNotFound)
}
