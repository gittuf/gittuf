// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/stretchr/testify/assert"
)

func TestPrepareRSLLogOutput(t *testing.T) {
	t.Run("simple without number", func(t *testing.T) {
		branchEntry := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash)
		branchEntry.ID = gitinterface.ZeroHash
		tagEntry := rsl.NewReferenceEntry("refs/tags/v1", gitinterface.ZeroHash)
		tagEntry.ID = gitinterface.ZeroHash

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

	t.Run("simple with number", func(t *testing.T) {
		branchEntry := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash)
		branchEntry.ID = gitinterface.ZeroHash
		branchEntry.Number = 1
		tagEntry := rsl.NewReferenceEntry("refs/tags/v1", gitinterface.ZeroHash)
		tagEntry.ID = gitinterface.ZeroHash
		tagEntry.Number = 2

		expectedOutput := `entry 0000000000000000000000000000000000000000

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1

entry 0000000000000000000000000000000000000000

  Ref:    refs/tags/v1
  Target: 0000000000000000000000000000000000000000
  Number: 2

`

		logOutput := PrepareRSLLogOutput([]*rsl.ReferenceEntry{branchEntry, tagEntry}, nil)
		assert.Equal(t, expectedOutput, logOutput)
	})

	t.Run("with annotations", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, true)

		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		if err := rsl.NewReferenceEntry("refs/tags/v1", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		branchEntry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference("refs/heads/main"))
		if err != nil {
			t.Fatal(err)
		}
		tagEntry, _, err := rsl.GetLatestReferenceEntry(repo, rsl.ForReference("refs/tags/v1"))
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{branchEntry.ID}, true, "msg").Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		annotationEntry, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		expectedOutput := fmt.Sprintf(`entry %s

  Ref:    refs/tags/v1
  Target: 0000000000000000000000000000000000000000
  Number: 2

entry %s (skipped)

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1

    Annotation ID: %s
    Skip:          yes
    Number:        3
    Message:
      msg

`, tagEntry.ID.String(), branchEntry.ID.String(), annotationEntry.GetID().String())

		logOutput := PrepareRSLLogOutput([]*rsl.ReferenceEntry{tagEntry, branchEntry}, map[string][]*rsl.AnnotationEntry{branchEntry.ID.String(): {annotationEntry.(*rsl.AnnotationEntry)}})
		assert.Equal(t, expectedOutput, logOutput)
	})
}
