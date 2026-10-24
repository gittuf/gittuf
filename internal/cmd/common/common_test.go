// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package common

import (
	"testing"

	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPublicKeys(t *testing.T) {
	t.Parallel()

	pk := &PublicKeys{}

	// Test Type()
	assert.Equal(t, "public-keys", pk.Type())

	// Test Set() and String() with initial value
	err := pk.Set("key1.pub")
	require.NoError(t, err)
	assert.Equal(t, "key1.pub", pk.String())

	// Test Set() appending a second value and String() formatting with comma separation
	err = pk.Set("key2.pub")
	require.NoError(t, err)
	assert.Equal(t, "key1.pub, key2.pub", pk.String())
}

func TestResolveRevision(t *testing.T) {
	tmpDir := t.TempDir()
	repo := gitinterface.CreateTestGitRepository(t, tmpDir, false)

	emptyTree, err := repo.EmptyTree()
	require.NoError(t, err)

	commitID, err := repo.Commit(emptyTree, "refs/heads/main", "Initial commit", false)
	require.NoError(t, err)

	_, err = repo.Commit(emptyTree, "refs/gittuf/policy-staging", "Policy commit", false)
	require.NoError(t, err)

	// 1. Full 40/64 character hash
	resolved, err := ResolveRevision(repo, commitID.String())
	require.NoError(t, err)
	assert.Equal(t, commitID, resolved)

	// 2. Short 7-character hash
	resolved, err = ResolveRevision(repo, commitID.String()[:7])
	require.NoError(t, err)
	assert.Equal(t, commitID, resolved)

	// 3. Git ref (e.g., refs/heads/main) must be rejected
	_, err = ResolveRevision(repo, "refs/heads/main")
	assert.ErrorIs(t, err, ErrInvalidRevision)

	// 4. Git symbolic ref / HEAD must be rejected
	_, err = ResolveRevision(repo, "HEAD")
	assert.ErrorIs(t, err, ErrInvalidRevision)

	// 5. Full gittuf ref must be rejected
	_, err = ResolveRevision(repo, "refs/gittuf/policy-staging")
	assert.ErrorIs(t, err, ErrInvalidRevision)

	// 6. Short gittuf policy ref name without refs/gittuf/ prefix must be rejected
	_, err = ResolveRevision(repo, "policy-staging")
	assert.ErrorIs(t, err, ErrInvalidRevision)

	// 7. Hash too short (< 4 chars)
	_, err = ResolveRevision(repo, "abc")
	assert.ErrorIs(t, err, ErrInvalidRevision)

	// 8. Non-existent short hash
	_, err = ResolveRevision(repo, "0000000")
	assert.ErrorContains(t, err, "unable to resolve revision '0000000'")

	// 9. Non-existent non-hex revision string
	_, err = ResolveRevision(repo, "non-existent-revision")
	assert.ErrorIs(t, err, ErrInvalidRevision)
}
