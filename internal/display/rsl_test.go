// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package display

import (
	"bytes"
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/rsl"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
)

func TestRSLLog(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	// add first entry
	if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo)
	if err != nil {
		t.Fatal(err)
	}

	// skip annotation
	if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, true, "msg").Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// add another entry
	if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// add another entry
	if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	entry, _, err = rsl.GetLatestReferenceUpdaterEntry(repo)
	if err != nil {
		t.Fatal(err)
	}

	// skip annotation
	if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, true, "msg").Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	// non-skip annotation
	if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "msg").Commit(repo, false); err != nil {
		t.Fatal(err)
	}

	expectedOutput := `entry 2d21a6b9fb1f3e432e0776eac63acdc23a57b538 (skipped)

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 4

    Annotation ID: 630618d8f80714658fb6d88bc352f92189d1d443
    Skip:          no
    Number:        6
    Message:
      msg

    Annotation ID: 15f60db9f339375f709dae8d04e0055ea50ed2b9
    Skip:          yes
    Number:        5
    Message:
      msg

entry ba2a366ccd85b3a4a636641c3604ce2d1496c08c

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 3

entry ae4467eaa656782fe9d04eaabfa30db47e9ea24b (skipped)

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1

    Annotation ID: f79156492abec45bb2e1dbc518999a83b31a069c
    Skip:          yes
    Number:        2
    Message:
      msg
`

	output := &bytes.Buffer{}
	writer := &noopwritecloser{writer: output}
	err = RSLLog(repo, writer)
	assert.Nil(t, err)
	assert.Equal(t, expectedOutput, output.String())
}

func TestWriteRSLReferenceEntry(t *testing.T) {
	// Set colorer to off for tests
	colorer = colorerOff

	t.Run("simple without number, no parent", func(t *testing.T) {
		entry := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash)
		entry.ID = gitinterface.ZeroHash

		expectedOutput := `entry 0000000000000000000000000000000000000000

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
`

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err := writeRSLReferenceEntry(testWriter, entry, nil, false)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("simple without number, has parent", func(t *testing.T) {
		entry := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash)
		entry.ID = gitinterface.ZeroHash

		expectedOutput := `entry 0000000000000000000000000000000000000000

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000

`

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err := writeRSLReferenceEntry(testWriter, entry, nil, true)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("simple with number, no parent", func(t *testing.T) {
		entry := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash)
		entry.ID = gitinterface.ZeroHash
		entry.Number = 1
		expectedOutput := `entry 0000000000000000000000000000000000000000

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1
`

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err := writeRSLReferenceEntry(testWriter, entry, nil, false)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("simple with number, has parent", func(t *testing.T) {
		entry := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash)
		entry.ID = gitinterface.ZeroHash
		entry.Number = 1
		expectedOutput := `entry 0000000000000000000000000000000000000000

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1

`

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err := writeRSLReferenceEntry(testWriter, entry, nil, true)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("with skip annotation, no parent", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, true)

		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, true, "msg").Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		annotationEntryT, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		annotationEntry := annotationEntryT.(*rsl.AnnotationEntry)

		expectedOutput := fmt.Sprintf(`entry %s (skipped)

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1

    Annotation ID: %s
    Skip:          yes
    Number:        2
    Message:
      msg
`, entry.GetID().String(), annotationEntry.GetID().String())

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err = writeRSLReferenceEntry(testWriter, entry.(*rsl.ReferenceEntry), []*rsl.AnnotationEntry{annotationEntry}, false)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("with skip annotation, has parent", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, true)

		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, true, "msg").Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		annotationEntryT, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		annotationEntry := annotationEntryT.(*rsl.AnnotationEntry)

		expectedOutput := fmt.Sprintf(`entry %s (skipped)

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1

    Annotation ID: %s
    Skip:          yes
    Number:        2
    Message:
      msg

`, entry.GetID().String(), annotationEntry.GetID().String())

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err = writeRSLReferenceEntry(testWriter, entry.(*rsl.ReferenceEntry), []*rsl.AnnotationEntry{annotationEntry}, true)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("with non-skip annotation, no parent", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, true)

		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "msg").Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		annotationEntryT, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		annotationEntry := annotationEntryT.(*rsl.AnnotationEntry)

		expectedOutput := fmt.Sprintf(`entry %s

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1

    Annotation ID: %s
    Skip:          no
    Number:        2
    Message:
      msg
`, entry.GetID().String(), annotationEntry.GetID().String())

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err = writeRSLReferenceEntry(testWriter, entry.(*rsl.ReferenceEntry), []*rsl.AnnotationEntry{annotationEntry}, false)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("with non-skip annotation, has parent", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, true)

		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "msg").Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		annotationEntryT, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		annotationEntry := annotationEntryT.(*rsl.AnnotationEntry)

		expectedOutput := fmt.Sprintf(`entry %s

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1

    Annotation ID: %s
    Skip:          no
    Number:        2
    Message:
      msg

`, entry.GetID().String(), annotationEntry.GetID().String())

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err = writeRSLReferenceEntry(testWriter, entry.(*rsl.ReferenceEntry), []*rsl.AnnotationEntry{annotationEntry}, true)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("with non-skip annotation, no parent, annotation message has trailing newline", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, true)

		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "msg\n").Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		annotationEntryT, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		annotationEntry := annotationEntryT.(*rsl.AnnotationEntry)

		expectedOutput := fmt.Sprintf(`entry %s

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1

    Annotation ID: %s
    Skip:          no
    Number:        2
    Message:
      msg
`, entry.GetID().String(), annotationEntry.GetID().String())

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err = writeRSLReferenceEntry(testWriter, entry.(*rsl.ReferenceEntry), []*rsl.AnnotationEntry{annotationEntry}, false)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("with non-skip annotation, has parent, annotation message has trailing newline", func(t *testing.T) {
		tmpDir := t.TempDir()
		repo := gitinterface.CreateTestGitRepository(t, tmpDir, true)

		if err := rsl.NewReferenceEntry("refs/heads/main", gitinterface.ZeroHash).Commit(repo, false); err != nil {
			t.Fatal(err)
		}

		entry, _, err := rsl.GetLatestReferenceUpdaterEntry(repo)
		if err != nil {
			t.Fatal(err)
		}

		if err := rsl.NewAnnotationEntry([]gitinterface.Hash{entry.GetID()}, false, "msg\n").Commit(repo, false); err != nil {
			t.Fatal(err)
		}
		annotationEntryT, err := rsl.GetLatestEntry(repo)
		if err != nil {
			t.Fatal(err)
		}
		annotationEntry := annotationEntryT.(*rsl.AnnotationEntry)

		expectedOutput := fmt.Sprintf(`entry %s

  Ref:    refs/heads/main
  Target: 0000000000000000000000000000000000000000
  Number: 1

    Annotation ID: %s
    Skip:          no
    Number:        2
    Message:
      msg

`, entry.GetID().String(), annotationEntry.GetID().String())

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err = writeRSLReferenceEntry(testWriter, entry.(*rsl.ReferenceEntry), []*rsl.AnnotationEntry{annotationEntry}, true)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})
}

func TestWriteRSLPropagationEntry(t *testing.T) {
	// Set colorer to off for tests
	colorer = colorerOff

	t.Run("simple, without number, without parent", func(t *testing.T) {
		entry := rsl.NewPropagationEntry("refs/heads/main", gitinterface.ZeroHash, "https://git.example.com/repository", gitinterface.ZeroHash)
		entry.ID = gitinterface.ZeroHash

		expectedOutput := `propagation entry 0000000000000000000000000000000000000000

  Ref:           refs/heads/main
  Target:        0000000000000000000000000000000000000000
  UpstreamRepo:  https://git.example.com/repository
  UpstreamEntry: 0000000000000000000000000000000000000000
`

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err := writeRSLPropagationEntry(testWriter, entry, false)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("simple, with number, without parent", func(t *testing.T) {
		entry := rsl.NewPropagationEntry("refs/heads/main", gitinterface.ZeroHash, "https://git.example.com/repository", gitinterface.ZeroHash)
		entry.Number = 1
		entry.ID = gitinterface.ZeroHash

		expectedOutput := `propagation entry 0000000000000000000000000000000000000000

  Ref:           refs/heads/main
  Target:        0000000000000000000000000000000000000000
  UpstreamRepo:  https://git.example.com/repository
  UpstreamEntry: 0000000000000000000000000000000000000000
  Number:        1
`

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err := writeRSLPropagationEntry(testWriter, entry, false)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("simple, without number, with parent", func(t *testing.T) {
		entry := rsl.NewPropagationEntry("refs/heads/main", gitinterface.ZeroHash, "https://git.example.com/repository", gitinterface.ZeroHash)
		entry.ID = gitinterface.ZeroHash

		expectedOutput := `propagation entry 0000000000000000000000000000000000000000

  Ref:           refs/heads/main
  Target:        0000000000000000000000000000000000000000
  UpstreamRepo:  https://git.example.com/repository
  UpstreamEntry: 0000000000000000000000000000000000000000

`

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err := writeRSLPropagationEntry(testWriter, entry, true)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})

	t.Run("simple, with number, with parent", func(t *testing.T) {
		entry := rsl.NewPropagationEntry("refs/heads/main", gitinterface.ZeroHash, "https://git.example.com/repository", gitinterface.ZeroHash)
		entry.Number = 1
		entry.ID = gitinterface.ZeroHash

		expectedOutput := `propagation entry 0000000000000000000000000000000000000000

  Ref:           refs/heads/main
  Target:        0000000000000000000000000000000000000000
  UpstreamRepo:  https://git.example.com/repository
  UpstreamEntry: 0000000000000000000000000000000000000000
  Number:        1

`

		output := &bytes.Buffer{}
		testWriter := &noopwritecloser{writer: output}
		err := writeRSLPropagationEntry(testWriter, entry, true)
		assert.Nil(t, err)
		assert.Equal(t, expectedOutput, output.String())
	})
}
