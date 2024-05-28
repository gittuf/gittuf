// SPDX-License-Identifier: Apache-2.0

package tuf

import (
	"fmt"
	"testing"
	"time"

	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/stretchr/testify/assert"
)

var (
	customEncodedPublicKeyBytes = artifacts.SSLibKey1Public
	pemEncodedPublicKeyBytes    = artifacts.SSHRSAPublic
)

func TestLoadKeyFromBytes(t *testing.T) {
	tests := map[string]struct {
		keyBytes          []byte
		expectedPublicKey string
		expectedKeyID     string
	}{
		"legacy serialization format key": {
			keyBytes:          customEncodedPublicKeyBytes,
			expectedKeyID:     "52e3b8e73279d6ebdd62a5016e2725ff284f569665eb92ccb145d83817a02997",
			expectedPublicKey: "3f586ce67329419fb0081bd995914e866a7205da463d593b3b490eab2b27fd3f",
		},
		"PEM encoded key": {
			keyBytes:      pemEncodedPublicKeyBytes,
			expectedKeyID: "be1d8bc05c5d287d72ecc5d96e7a32858c377d940bd09ff86f228b09b90ff6cf",
			expectedPublicKey: `-----BEGIN PUBLIC KEY-----
MIIBojANBgkqhkiG9w0BAQEFAAOCAY8AMIIBigKCAYEAxCOK3QmP8wN6DjHrdSWC
flboFGqTX4+4XAGXptQkbrHeRU4llZjYeMssIAPtSVg7AZ8i32zmQYs+H4kEPvm0
Rf5g10MdkK9sYAktu7Tcsr/LXxERroJQLuvU+2i7Hzxp+FgmlV9/jrgC2VWkJfxe
TM6A5ImmZArAa1WZHy99/d2ZYbHm/22PO3+1DsAK6t1jDCvUKdHDvP/K7GZ87t5x
QpWEiCylMe1a3+7kmlLZd/Q4f0DF9ZugMdDIeN4DJ5PhW64WovgVqN2OfAOfiwVC
946g5Jl8xb4WCYBiiElOZ3yghww6pboPpecCZhrrNRvI2ebmO7NmR0ucrpj0DaAi
Gc7Zi5tA4EKjTeCfmLm+fi+sgoANUOpiqv+t9fw8h7xfHuRpZwjm9cxO82iWnCeN
rTXVau2d8fh4pPT0X4AT7daBeNUhUgqdGETy8SzBXU4zYFdx2V1TZauuiIcqA2do
Y3eV5jJqu4dWvCMmwbCLSfCJJOF9dryMwkqjpcpWUuPlAgMBAAE=
-----END PUBLIC KEY-----`,
		},
	}

	for name, test := range tests {
		key, err := LoadKeyFromBytes(test.keyBytes)
		assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
		assert.Equal(t, test.expectedPublicKey, key.KeyVal.Public)
		assert.Equal(t, test.expectedKeyID, key.KeyID)
	}
}

func TestRootMetadata(t *testing.T) {
	rootMetadata := NewRootMetadata()

	t.Run("test NewRootMetadata", func(t *testing.T) {
		assert.Equal(t, specVersion, rootMetadata.SpecVersion)
		assert.Equal(t, 0, rootMetadata.Version)
	})

	t.Run("test SetVersion", func(t *testing.T) {
		rootMetadata.SetVersion(10)
		assert.Equal(t, 10, rootMetadata.Version)
	})

	t.Run("test SetExpires", func(t *testing.T) {
		d := time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC)
		rootMetadata.SetExpires(d.Format(time.RFC3339))
		assert.Equal(t, "1995-10-26T09:00:00Z", rootMetadata.Expires)
	})

	key, err := LoadKeyFromBytes(customEncodedPublicKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

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

	t.Run("test NewTargetsMetadata", func(t *testing.T) {
		assert.Equal(t, specVersion, targetsMetadata.SpecVersion)
		assert.Equal(t, 0, targetsMetadata.Version)
	})

	t.Run("test SetVersion", func(t *testing.T) {
		targetsMetadata.SetVersion(10)
		assert.Equal(t, 10, targetsMetadata.Version)
	})

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

	key, err := LoadKeyFromBytes(customEncodedPublicKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

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
			patterns: []string{"foo/**"},
			target:   "foo/bar/foobar",
			expected: true,
		},
		"artifact in arbitrary directory, matches": {
			patterns: []string{"**/*.txt"},
			target:   "foo/bar/foobar.txt",
			expected: true,
		},
		"artifact with specific name in arbitrary directory, matches": {
			patterns: []string{"**/foobar.txt"},
			target:   "foo/bar/foobar.txt",
			expected: true,
		},
		"artifact with arbitrary subdirectories, matches": {
			patterns: []string{"foo/**/foobar.txt"},
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
			patterns: []string{"file:src/signatures/**"},
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
