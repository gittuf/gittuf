// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tuf

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

var testKey = artifacts.SSHRSAPublicSSH

func TestRootMetadata(t *testing.T) {
	rootMetadata := NewRootMetadata()

	t.Run("test SetExpires", func(t *testing.T) {
		d := time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC)
		rootMetadata.SetExpires(d.Format(time.RFC3339))
		assert.Equal(t, "1995-10-26T09:00:00Z", rootMetadata.Expires)
	})

	key := ssh.NewKeyFromBytes(t, testKey)

	t.Run("test AddKey", func(t *testing.T) {
		rootMetadata.AddKey(key)
		assert.Equal(t, key, rootMetadata.Keys[key.KeyID])
	})

	t.Run("test AddRole", func(t *testing.T) {
		rootMetadata.AddRole("targets", Role{
			KeyIDs:    []string{key.KeyID},
			Threshold: 1,
		})
		assert.Contains(t, rootMetadata.Roles["targets"].KeyIDs, key.KeyID)
	})
}

func TestTargetsMetadataAndDelegations(t *testing.T) {
	targetsMetadata := NewTargetsMetadata()

	t.Run("test SetExpires", func(t *testing.T) {
		d := time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC)
		targetsMetadata.SetExpires(d.Format(time.RFC3339))
		assert.Equal(t, "1995-10-26T09:00:00Z", targetsMetadata.Expires)
	})

	t.Run("test Validate", func(t *testing.T) {
		err := targetsMetadata.Validate()
		assert.Nil(t, err)

		targetsMetadata.Targets = map[string]any{"test": true}
		err = targetsMetadata.Validate()
		assert.ErrorIs(t, err, ErrTargetsNotEmpty)
		targetsMetadata.Targets = nil
	})

	key := ssh.NewKeyFromBytes(t, testKey)

	delegations := &Delegations{}

	t.Run("test AddKey", func(t *testing.T) {
		assert.Nil(t, delegations.Keys)
		delegations.AddKey(key)
		assert.Equal(t, key, delegations.Keys[key.KeyID])
	})

	t.Run("test AddDelegation", func(t *testing.T) {
		assert.Nil(t, delegations.Roles)
		d := Delegation{
			Name: "delegation",
			Role: Role{
				KeyIDs:    []string{key.KeyID},
				Threshold: 1,
			},
		}
		delegations.AddDelegation(d)
		assert.Contains(t, delegations.Roles, d)
	})
}

func TestDelegationMatches(t *testing.T) {
	tests := map[string]struct {
		patterns []string
		target   string
		expected bool
	}{
		"full path, matches": {
			patterns: []string{"foo"},
			target:   "foo",
			expected: true,
		},
		"artifact in directory, matches": {
			patterns: []string{"foo/*"},
			target:   "foo/bar",
			expected: true,
		},
		"artifact in directory, does not match": {
			patterns: []string{"foo/*.txt"},
			target:   "foo/bar.tgz",
			expected: false,
		},
		"artifact in directory, one pattern matches": {
			patterns: []string{"foo/*.txt", "foo/*.tgz"},
			target:   "foo/bar.tgz",
			expected: true,
		},
		"artifact in subdirectory, matches": {
			patterns: []string{"foo/*"},
			target:   "foo/bar/foobar",
			expected: true,
		},
		"artifact in subdirectory with specified extension, matches": {
			patterns: []string{"foo/*.tgz"},
			target:   "foo/bar/foobar.tgz",
			expected: true,
		},
		"pattern with single character selector, matches": {
			patterns: []string{"foo/?.tgz"},
			target:   "foo/a.tgz",
			expected: true,
		},
		"pattern with character sequence, matches": {
			patterns: []string{"foo/[abc].tgz"},
			target:   "foo/a.tgz",
			expected: true,
		},
		"pattern with character sequence, does not match": {
			patterns: []string{"foo/[abc].tgz"},
			target:   "foo/x.tgz",
			expected: false,
		},
		"pattern with negative character sequence, matches": {
			patterns: []string{"foo/[!abc].tgz"},
			target:   "foo/x.tgz",
			expected: true,
		},
		"pattern with negative character sequence, does not match": {
			patterns: []string{"foo/[!abc].tgz"},
			target:   "foo/a.tgz",
			expected: false,
		},
		"artifact in arbitrary directory, matches": {
			patterns: []string{"*/*.txt"},
			target:   "foo/bar/foobar.txt",
			expected: true,
		},
		"artifact with specific name in arbitrary directory, matches": {
			patterns: []string{"*/foobar.txt"},
			target:   "foo/bar/foobar.txt",
			expected: true,
		},
		"artifact with arbitrary subdirectories, matches": {
			patterns: []string{"foo/*/foobar.txt"},
			target:   "foo/bar/baz/foobar.txt",
			expected: true,
		},
		"artifact in arbitrary directory, does not match": {
			patterns: []string{"*.txt"},
			target:   "foo/bar/foobar.txtfile",
			expected: false,
		},
		"arbitrary directory, does not match": {
			patterns: []string{"*_test"},
			target:   "foo/bar_test/foobar",
			expected: false,
		},
		"no patterns": {
			patterns: nil,
			target:   "foo",
			expected: false,
		},
		"pattern with multiple consecutive wildcards, matches": {
			patterns: []string{"foo/*/*/*.txt"},
			target:   "foo/bar/baz/qux.txt",
			expected: true,
		},
		"pattern with multiple non-consecutive wildcards, matches": {
			patterns: []string{"foo/*/baz/*.txt"},
			target:   "foo/bar/baz/qux.txt",
			expected: true,
		},
		"pattern with gittuf git prefix, matches": {
			patterns: []string{"git:refs/heads/*"},
			target:   "git:refs/heads/main",
			expected: true,
		},
		"pattern with gittuf file prefix for all recursive contents, matches": {
			patterns: []string{"file:src/signatures/*"},
			target:   "file:src/signatures/rsa/rsa.go",
			expected: true,
		},
	}

	for name, test := range tests {
		delegation := Delegation{Paths: test.patterns}
		got := delegation.Matches(test.target)
		assert.Equal(t, test.expected, got, fmt.Sprintf("unexpected result in test '%s'", name))
	}
}

func TestRootMetadataWithSSHKey(t *testing.T) {
	// Setup test key pair
	keys := []struct {
		name string
		data []byte
	}{
		{"rsa", artifacts.SSHRSAPrivate},
		{"rsa.pub", artifacts.SSHRSAPublicSSH},
	}
	tmpDir := t.TempDir()
	for _, key := range keys {
		keyPath := filepath.Join(tmpDir, key.name)
		if err := os.WriteFile(keyPath, key.data, 0o600); err != nil {
			t.Fatal(err)
		}
	}
	keyPath := filepath.Join(tmpDir, "rsa")
	sslibKey, err := ssh.NewKeyFromFile(keyPath)
	if err != nil {
		t.Fatal()
	}

	// Create TUF root and add test key
	rootMetadata := NewRootMetadata()
	rootMetadata.AddKey(sslibKey)

	// Wrap and and sign
	ctx := context.Background()
	env, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		t.Fatal()
	}

	verifier, err := ssh.NewVerifierFromKey(sslibKey)
	if err != nil {
		t.Fatal()
	}
	signer := &ssh.Signer{
		Verifier: verifier,
		Path:     keyPath,
	}

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		t.Fatal()
	}
	// Unwrap and verify
	// NOTE: For the sake of testing the contained key, we unwrap before we
	// verify. Typically, in DSSE it should be the other way around.
	payload, err := env.DecodeB64Payload()
	if err != nil {
		t.Fatal()
	}
	rootMetadata2 := &RootMetadata{}
	if err := json.Unmarshal(payload, rootMetadata2); err != nil {
		t.Fatal()
	}

	sslibKey2 := rootMetadata2.Keys[sslibKey.KeyID]

	// NOTE: Typically, a caller would choose this method, if KeyType==ssh.SSHKeyType
	verifier2, err := ssh.NewVerifierFromKey(sslibKey2)
	if err != nil {
		t.Fatal()
	}
	_, err = dsse.VerifyEnvelope(ctx, env, []sslibdsse.Verifier{verifier2}, 1)
	if err != nil {
		t.Fatal()
	}
}
