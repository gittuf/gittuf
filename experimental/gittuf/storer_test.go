// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"testing"

	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestListPrincipalsBackendEquivalenceAndForks(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")
	resetStorerForTest(t)
	require.Nil(t, SetStorerBackend(StorerBackendBinary))
	repo := createTestRepositoryWithPolicyWithFileRule(t, "")
	binary := repo.GetGitRepository()
	before := binary.GitInvocationCount()
	want, err := repo.ListPrincipals(t.Context(), policy.PolicyRef, tuf.TargetsRoleName)
	require.NoError(t, err)
	binaryForks := binary.GitInvocationCount() - before

	require.Nil(t, SetStorerBackend(StorerBackendGoGit))
	fast := &Repository{r: newStorer(binary)}
	before = binary.GitInvocationCount()
	got, err := fast.ListPrincipals(t.Context(), policy.PolicyRef, tuf.TargetsRoleName)
	require.NoError(t, err)
	fastForks := binary.GitInvocationCount() - before

	assert.Equal(t, want, got)
	assert.Less(t, fastForks, binaryForks)
}

// resetStorerForTest restores the process wide selection afterwards.
func resetStorerForTest(t *testing.T) {
	t.Helper()

	previous := storer
	t.Cleanup(func() {
		storer = previous
		activeTrace.mu.Lock()
		activeTrace.trace, activeTrace.repo = nil, nil
		activeTrace.mu.Unlock()
	})
	storer = &storerSelection{warn: func(string) {}, debugf: func(string) {}}
}

func TestParseStorerBackend(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		value       string
		expected    StorerBackend
		expectedErr bool
	}{
		"binary":                   {value: "binary", expected: StorerBackendBinary},
		"go-git":                   {value: "go-git", expected: StorerBackendGoGit},
		"empty defaults to binary": {value: "", expected: StorerBackendBinary},
		"unknown":                  {value: "libgit2", expectedErr: true},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			backend, err := ParseStorerBackend(test.value)
			if test.expectedErr {
				assert.ErrorIs(t, err, ErrUnknownStorerBackend)
				return
			}
			require.Nil(t, err)
			assert.Equal(t, test.expected, backend)
		})
	}
}

func TestStorerSelection(t *testing.T) {
	t.Run("warns once when go-git is selected", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "1")

		warnings := []string{}
		selection := &storerSelection{warn: func(message string) { warnings = append(warnings, message) }}
		selection.set(StorerBackendGoGit)

		assert.Equal(t, StorerBackendGoGit, selection.resolve())
		assert.Equal(t, StorerBackendGoGit, selection.resolve())
		assert.Equal(t, StorerBackendGoGit, selection.resolve())

		require.Len(t, warnings, 1)
		assert.Contains(t, warnings[0], "EXPERIMENTAL")
	})
}

func TestStorerSelectionFromEnvironment(t *testing.T) {
	t.Run("defaults to the git binary and does not warn", func(t *testing.T) {
		t.Setenv(StorerBackendEnvKey, "")

		warnings := []string{}
		selection := &storerSelection{warn: func(message string) { warnings = append(warnings, message) }}

		assert.Equal(t, StorerBackendBinary, selection.resolve())
		assert.Empty(t, warnings)
	})

	t.Run("environment selects the backend when nothing was set", func(t *testing.T) {
		selection := &storerSelection{warn: func(string) {}}
		t.Setenv(StorerBackendEnvKey, "go-git")
		t.Setenv(dev.DevModeKey, "1")

		assert.Equal(t, StorerBackendGoGit, selection.resolve())
	})

	t.Run("the environment cannot select go-git outside developer mode", func(t *testing.T) {
		selection := &storerSelection{warn: func(string) {}}
		t.Setenv(StorerBackendEnvKey, "go-git")
		t.Setenv(dev.DevModeKey, "")

		assert.Equal(t, StorerBackendBinary, selection.resolve())
	})

	t.Run("an explicit setting beats the environment", func(t *testing.T) {
		selection := &storerSelection{warn: func(string) {}}
		t.Setenv(StorerBackendEnvKey, "go-git")
		t.Setenv(dev.DevModeKey, "1")
		selection.set(StorerBackendBinary)

		assert.Equal(t, StorerBackendBinary, selection.resolve())
	})

	t.Run("an unparseable environment value falls back to the git binary", func(t *testing.T) {
		selection := &storerSelection{warn: func(string) {}}
		t.Setenv(StorerBackendEnvKey, "libgit2")

		assert.Equal(t, StorerBackendBinary, selection.resolve())
	})
}

func TestStorerSelectionLogsResolvedBackend(t *testing.T) {
	t.Setenv(dev.DevModeKey, "1")

	messages := []string{}
	selection := &storerSelection{
		warn:   func(string) {},
		debugf: func(message string) { messages = append(messages, message) },
	}
	selection.set(StorerBackendGoGit)

	require.Equal(t, StorerBackendGoGit, selection.resolve())
	require.Len(t, messages, 1)
	assert.Contains(t, messages[0], string(StorerBackendGoGit))
}

func TestStorerTraceReport(t *testing.T) {
	t.Run("absent unless tracing was enabled", func(t *testing.T) {
		t.Setenv(StorerBackendEnvKey, "")
		resetStorerForTest(t)

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		storer := newStorer(repo)

		_, err := storer.WriteBlob([]byte("contents"))
		require.Nil(t, err)

		_, has := StorerTraceReport()
		assert.False(t, has)
	})

	t.Run("reports storer calls and git forks once enabled", func(t *testing.T) {
		t.Setenv(StorerBackendEnvKey, "")
		t.Setenv(dev.DevModeKey, "1")
		resetStorerForTest(t)
		require.Nil(t, SetStorerBackend(StorerBackendGoGit))
		SetStorerTrace(true)

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		storer := newStorer(repo)

		blobID, err := storer.WriteBlob([]byte("contents"))
		require.Nil(t, err)
		_, err = storer.ReadBlob(blobID)
		require.Nil(t, err)

		report, has := StorerTraceReport()
		require.True(t, has)
		assert.Contains(t, report, "ReadBlob")
		assert.Contains(t, report, string(StorerBackendGoGit))
		assert.Contains(t, report, "git forks")
	})

	t.Run("disabling tracing discards the report already collected", func(t *testing.T) {
		t.Setenv(StorerBackendEnvKey, "")
		t.Setenv(dev.DevModeKey, "1")
		resetStorerForTest(t)
		require.Nil(t, SetStorerBackend(StorerBackendGoGit))
		SetStorerTrace(true)

		repo := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		storer := newStorer(repo)

		blobID, err := storer.WriteBlob([]byte("contents"))
		require.Nil(t, err)
		_, err = storer.ReadBlob(blobID)
		require.Nil(t, err)

		_, has := StorerTraceReport()
		require.True(t, has)

		SetStorerTrace(false)

		_, has = StorerTraceReport()
		assert.False(t, has, "the previous repository's metrics must not survive disabling")
	})
}

func TestSetStorerBackendValidatesTheBackend(t *testing.T) {
	t.Run("an unrecognised backend is rejected", func(t *testing.T) {
		t.Setenv(StorerBackendEnvKey, "")
		t.Setenv(dev.DevModeKey, "1")
		resetStorerForTest(t)

		assert.ErrorIs(t, SetStorerBackend(StorerBackend("libgit2")), ErrUnknownStorerBackend)
		assert.Equal(t, StorerBackendBinary, StorerBackendInUse())
	})

	t.Run("an empty backend is stored as the default", func(t *testing.T) {
		t.Setenv(StorerBackendEnvKey, "")
		t.Setenv(dev.DevModeKey, "1")
		resetStorerForTest(t)

		require.Nil(t, SetStorerBackend(StorerBackend("")))
		assert.Equal(t, StorerBackendBinary, StorerBackendInUse())
	})
}

func TestSetStorerBackendRequiresDevMode(t *testing.T) {
	t.Run("go-git is refused outside developer mode", func(t *testing.T) {
		t.Setenv(dev.DevModeKey, "")
		resetStorerForTest(t)

		assert.ErrorIs(t, SetStorerBackend(StorerBackendGoGit), dev.ErrNotInDevMode)
	})

	t.Run("the git binary backend needs no developer mode", func(t *testing.T) {
		t.Setenv(StorerBackendEnvKey, "")
		t.Setenv(dev.DevModeKey, "")
		resetStorerForTest(t)

		require.Nil(t, SetStorerBackend(StorerBackendBinary))
		assert.Equal(t, StorerBackendBinary, StorerBackendInUse())
	})
}
