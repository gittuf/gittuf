// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/stretchr/testify/assert"
)

func TestRegularSearcher(t *testing.T) {
	t.Run("policy exists", func(t *testing.T) {
		repo, _ := createTestRepository(t, createTestStateWithOnlyRoot)

		expectedPolicyEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		// Add an entry after
		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := NewRegularSearcher(repo)
		policyEntry, err := searcher.FindPolicyEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntry.GetID(), policyEntry.GetID())

		// Try with annotation
		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "Annotation\n").Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		policyEntry, err = searcher.FindPolicyEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedPolicyEntry.GetID(), policyEntry.GetID())

		// Requested entry is annotations entry
		if err := rsl.NewReferenceEntry(PolicyRef, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		policyEntry, err = searcher.FindPolicyEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, entry.GetID(), policyEntry.GetID())
	})

	t.Run("policy does not exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		// Add an entry after
		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		searcher := NewRegularSearcher(repo)
		policyEntry, err := searcher.FindPolicyEntryFor(entry)
		assert.ErrorIs(t, err, ErrPolicyNotFound)
		assert.Nil(t, policyEntry)
	})
}
