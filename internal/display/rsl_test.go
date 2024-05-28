// SPDX-License-Identifier: Apache-2.0

package display

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/go-git/go-billy/v5/memfs"
	"github.com/go-git/go-git/v5"
	"github.com/go-git/go-git/v5/plumbing"
	"github.com/go-git/go-git/v5/storage/memory"
	"github.com/stretchr/testify/assert"
)

func TestPrepareRSLLogOutput(t *testing.T) {
	t.Run("simple", func(t *testing.T) {
		branchEntry := rsl.NewReferenceEntry("refs/heads/main", plumbing.ZeroHash)
		tagEntry := rsl.NewReferenceEntry("refs/tags/v1", plumbing.ZeroHash)

		expectedOutput := `entry 0000000000000000000000000000000000000000

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000

entry 0000000000000000000000000000000000000000

  Ref:    refs/tags/v1
  Target: 0000000000000000000000000000000000000000
`

		logOutput := PrepareRSLLogOutput([]*rsl.ReferenceEntry{branchEntry, tagEntry}, nil)
		assert.Equal(t, expectedOutput, logOutput)
	})

	t.Run("with annotations", func(t *testing.T) {
		repo, err := git.Init(memory.NewStorage(), memfs.New())
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewReferenceEntry("refs/heads/main", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		if err := rsl.NewReferenceEntry("refs/tags/v1", plumbing.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		branchEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, "refs/heads/main")
		if err != nil {
			t.Fatal(err)
		}
		tagEntry, _, err := rsl.GetLatestReferenceEntryForRef(repo, "refs/tags/v1")
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewAnnotationEntry([]plumbing.Hash{branchEntry.ID}, true, "msg").Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		annotationEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		expectedOutput := fmt.Sprintf(`entry %s

  Ref:    refs/tags/v1
  Target: 0000000000000000000000000000000000000000

entry %s (skipped)

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000

    Annotation ID: %s
    Skip:          yes
    Message:
      msg
`, tagEntry.ID.String(), branchEntry.ID.String(), annotationEntry.GetID().String())

		logOutput := PrepareRSLLogOutput([]*rsl.ReferenceEntry{tagEntry, branchEntry}, map[plumbing.Hash][]*rsl.AnnotationEntry{branchEntry.ID: {annotationEntry.(*rsl.AnnotationEntry)}})
		assert.Equal(t, expectedOutput, logOutput)
	})
}
