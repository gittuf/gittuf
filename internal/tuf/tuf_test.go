package tuf

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestLoadKeyFromBytes(t *testing.T) {
	publicKeyPath := filepath.Join("test-data", "test-key.pub")
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	key, err := LoadKeyFromBytes(publicKeyBytes)
	if err != nil {
		t.Error(err)
	}
	assert.Equal(t, "3f586ce67329419fb0081bd995914e866a7205da463d593b3b490eab2b27fd3f", key.KeyVal.Public)
	assert.Equal(t, "52e3b8e73279d6ebdd62a5016e2725ff284f569665eb92ccb145d83817a02997", key.KeyID)
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

	publicKeyPath := filepath.Join("test-data", "test-key.pub")
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	key, err := LoadKeyFromBytes(publicKeyBytes)
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

	publicKeyPath := filepath.Join("test-data", "test-key.pub")
	publicKeyBytes, err := os.ReadFile(publicKeyPath)
	if err != nil {
		t.Fatal(err)
	}

	key, err := LoadKeyFromBytes(publicKeyBytes)
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

func TestNewDelegationPath(t *testing.T) {
	tests := map[string]struct {
		pattern                string
		expectedDelegationPath *DelegationPath
		expectedError          error
	}{
		"only Git ref": {
			pattern:                "git:refs/heads/main",
			expectedDelegationPath: &DelegationPath{GitRefPattern: "refs/heads/main", FilePattern: "*"},
		},
		"only File ref": {
			pattern:                "file:foo/*",
			expectedDelegationPath: &DelegationPath{GitRefPattern: "*", FilePattern: "foo/*"},
		},
		"both Git and File ref": {
			pattern:                "git:refs/heads/main&&file:foo/*",
			expectedDelegationPath: &DelegationPath{GitRefPattern: "refs/heads/main", FilePattern: "foo/*"},
		},
		"reversed Git and File ref": {
			pattern:                "file:foo/*&&git:refs/heads/main",
			expectedDelegationPath: &DelegationPath{GitRefPattern: "refs/heads/main", FilePattern: "foo/*"},
		},
		"empty pattern": {
			pattern:       "",
			expectedError: ErrInvalidDelegationPattern,
		},
		"incorrect Git prefix": {
			pattern:       "giit:refs/heads/main",
			expectedError: ErrInvalidDelegationPattern,
		},
		"incorrect File prefix": {
			pattern:       "fiile:foo/*",
			expectedError: ErrInvalidDelegationPattern,
		},
		"incorrect Git prefix, correct File prefix": {
			pattern:       "giit:refs/heads/main&&file:foo/*",
			expectedError: ErrInvalidDelegationPattern,
		},
		"incorrect File prefix, correct Git prefix": {
			pattern:       "git:refs/heads/main&&fiile:foo/*",
			expectedError: ErrInvalidDelegationPattern,
		},
	}

	for name, test := range tests {
		delegationPath, err := NewDelegationPath(test.pattern)
		if err != nil {
			assert.ErrorIs(t, err, test.expectedError, fmt.Sprintf("unexpected error in test %s", name))
		} else {
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test %s", name))
			assert.Equal(t, test.expectedDelegationPath, delegationPath, fmt.Sprintf("unexpected delegation path in test %s", name))
		}
	}
}

func TestDelegationPathMatches(t *testing.T) {
	tests := map[string]struct {
		delegationPath  *DelegationPath
		gitRef          string
		file            string
		expectedMatched bool
	}{
		"only Git pattern, matches": {
			delegationPath:  &DelegationPath{GitRefPattern: "refs/heads/main", FilePattern: "*"},
			gitRef:          "refs/heads/main",
			file:            "foo",
			expectedMatched: true,
		},
		"only File pattern, matches": {
			delegationPath:  &DelegationPath{GitRefPattern: "*", FilePattern: "foo"},
			gitRef:          "refs/heads/main",
			file:            "foo",
			expectedMatched: true,
		},
	}

	for name, test := range tests {
		matched := test.delegationPath.Matches(test.gitRef, test.file)
		assert.Equal(t, test.expectedMatched, matched, fmt.Sprintf("unexpected matched result in test %s", name))
	}
}
