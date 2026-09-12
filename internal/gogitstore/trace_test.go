// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gogitstore_test

import (
	"testing"

	"github.com/gittuf/gittuf/internal/gogitstore"
	"github.com/gittuf/gittuf/pkg/githash"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/gitstore"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTraceRecordsCalls(t *testing.T) {
	t.Parallel()

	setup := func(t *testing.T) (*gogitstore.Storer, *gogitstore.Trace, githash.Hash) {
		t.Helper()

		binary := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
		trace := gogitstore.NewTrace()
		blobID, err := binary.WriteBlob([]byte("contents"))
		require.Nil(t, err)

		return gogitstore.NewWithTrace(binary, true, trace), trace, blobID
	}

	t.Run("in-process read", func(t *testing.T) {
		t.Parallel()

		fast, trace, blobID := setup(t)

		_, err := fast.ReadBlob(blobID)
		require.Nil(t, err)

		stat := trace.Stat("ReadBlob")
		assert.Equal(t, 1, stat.Calls)
		assert.Equal(t, 0, stat.Delegated)
		assert.Positive(t, stat.Total)
	})

	t.Run("a method with no go-git implementation counts as delegated", func(t *testing.T) {
		t.Parallel()

		fast, trace, _ := setup(t)

		_, _, err := fast.LookupConfig(gitstore.ConfigUserName)
		require.Nil(t, err)

		stat := trace.Stat("LookupConfig")
		assert.Equal(t, 1, stat.Calls)
		assert.Equal(t, 1, stat.Delegated)
	})

	t.Run("repeated calls accumulate", func(t *testing.T) {
		t.Parallel()

		fast, trace, blobID := setup(t)

		for range 4 {
			_, err := fast.ReadBlob(blobID)
			require.Nil(t, err)
		}

		assert.Equal(t, 4, trace.Stat("ReadBlob").Calls)
	})
}

func TestTraceWithBackendDisabled(t *testing.T) {
	t.Parallel()

	binary := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	trace := gogitstore.NewTrace()
	disabled := gogitstore.NewWithTrace(binary, false, trace)

	blobID, err := binary.WriteBlob([]byte("contents"))
	require.Nil(t, err)
	_, err = disabled.ReadBlob(blobID)
	require.Nil(t, err)

	stat := trace.Stat("ReadBlob")
	assert.Equal(t, 1, stat.Calls)
	assert.Equal(t, 1, stat.Delegated)
	assert.Zero(t, trace.HandleOpens(), "the go-git handle must never open when the backend is off")
}

func TestTraceCountsHandleOpens(t *testing.T) {
	t.Parallel()

	binary := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	trace := gogitstore.NewTrace()
	fast := gogitstore.NewWithTrace(binary, true, trace)

	blobID, err := binary.WriteBlob([]byte("contents"))
	require.Nil(t, err)

	for range 3 {
		_, err := fast.ReadBlob(blobID)
		require.Nil(t, err)
	}
	assert.Equal(t, 1, trace.HandleOpens(), "the handle is cached across reads")

	_, err = fast.WriteBlob([]byte("more contents"))
	require.Nil(t, err)
	_, err = fast.ReadBlob(blobID)
	require.Nil(t, err)

	assert.Equal(t, 2, trace.HandleOpens())
}

func TestTraceReport(t *testing.T) {
	t.Parallel()

	binary := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)
	trace := gogitstore.NewTrace()
	fast := gogitstore.NewWithTrace(binary, true, trace)

	blobID, err := binary.WriteBlob([]byte("contents"))
	require.Nil(t, err)
	_, err = fast.ReadBlob(blobID)
	require.Nil(t, err)

	report := trace.Report("go-git", 7)

	assert.Contains(t, report, "go-git")
	assert.Contains(t, report, "ReadBlob")
	assert.NotContains(t, report, "fast")
	assert.Contains(t, report, "git forks")
	assert.Contains(t, report, "7")
}

func TestGitInvocationCountRises(t *testing.T) {
	t.Parallel()

	binary := gitinterface.CreateTestGitRepository(t, t.TempDir(), false)

	before := binary.GitInvocationCount()
	_, err := binary.WriteBlob([]byte("contents"))
	require.Nil(t, err)

	assert.Greater(t, binary.GitInvocationCount(), before)
}
