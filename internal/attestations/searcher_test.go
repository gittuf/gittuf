// SPDX-License-Identifier: Apache-2.0

package attestations

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/stretchr/testify/assert"
)

func TestRegularSearcher(t *testing.T) {
	t.Run("attestations exist", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

		attestations := &Attestations{}
		if err := attestations.Commit(repo, "Initial attestations\n", false); err != nil {
			t.Fatal(err)
		}

		expectedAttestationsEntry, err := rsl.GetLatestEntry(repo)
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
		attestationsEntry, err := searcher.FindAttestationsEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedAttestationsEntry.GetID(), attestationsEntry.GetID())

		// Try with annotation
		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "Annotation\n").Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		attestationsEntry, err = searcher.FindAttestationsEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, expectedAttestationsEntry.GetID(), attestationsEntry.GetID())

		// Requested entry is annotations entry
		if err := rsl.NewReferenceEntry(Ref, gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, err = rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		attestationsEntry, err = searcher.FindAttestationsEntryFor(entry)
		assert.Nil(t, err)
		assert.Equal(t, entry.GetID(), attestationsEntry.GetID())
	})

	t.Run("attestations do not exist", func(t *testing.T) {
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
		attestationsEntry, err := searcher.FindAttestationsEntryFor(entry)
		assert.ErrorIs(t, err, ErrAttestationsNotFound)
		assert.Nil(t, attestationsEntry)
	})
}
