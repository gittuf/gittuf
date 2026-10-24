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

	policyCommitID, err := repo.Commit(emptyTree, "refs/gittuf/policy-staging", "Policy commit", false)
	require.NoError(t, err)

	// 1. Full 40/64 character hash
	resolved, err := ResolveRevision(repo, commitID.String())
	require.NoError(t, err)
	assert.Equal(t, commitID, resolved)

	// 2. Short 7-character hash
	resolved, err = ResolveRevision(repo, commitID.String()[:7])
	require.NoError(t, err)
	assert.Equal(t, commitID, resolved)

	// 3. Git ref (e.g., refs/heads/main)
	resolved, err = ResolveRevision(repo, "refs/heads/main")
	require.NoError(t, err)
	assert.Equal(t, commitID, resolved)

	// 4. Git symbolic ref / HEAD
	resolved, err = ResolveRevision(repo, "HEAD")
	require.NoError(t, err)
	assert.Equal(t, commitID, resolved)

	// 5. Full gittuf ref
	resolved, err = ResolveRevision(repo, "refs/gittuf/policy-staging")
	require.NoError(t, err)
	assert.Equal(t, policyCommitID, resolved)

	// 6. Short gittuf policy ref name without refs/gittuf/ prefix
	resolved, err = ResolveRevision(repo, "policy-staging")
	require.NoError(t, err)
	assert.Equal(t, policyCommitID, resolved)

	// 7. Non-existent revision
	_, err = ResolveRevision(repo, "non-existent-revision")
	assert.ErrorContains(t, err, "unable to resolve revision 'non-existent-revision'")
}
