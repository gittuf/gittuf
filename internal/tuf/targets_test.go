// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tuf

import (
	"fmt"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/stretchr/testify/assert"
)

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

	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

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
				KeyIDs:    set.NewSetFromItems(key.KeyID),
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

func TestAddDelegation(t *testing.T) {
	targetsMetadata := initialTestTargetsMetadata(t)

	key1 := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)
	key2 := ssh.NewKeyFromBytes(t, targets2PubKeyBytes)

	err := targetsMetadata.AddDelegation("test-rule", []*Key{key1, key2}, []string{"test/"}, 1)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, key1.KeyID)
	assert.Equal(t, key1, targetsMetadata.Delegations.Keys[key1.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Keys, key2.KeyID)
	assert.Equal(t, key2, targetsMetadata.Delegations.Keys[key2.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Equal(t, Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        Role{KeyIDs: set.NewSetFromItems(key1.KeyID, key2.KeyID), Threshold: 1},
	}, targetsMetadata.Delegations.Roles[0])
}

func TestUpdateDelegation(t *testing.T) {
	targetsMetadata := initialTestTargetsMetadata(t)

	key1 := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)
	key2 := ssh.NewKeyFromBytes(t, targets2PubKeyBytes)

	err := targetsMetadata.AddDelegation("test-rule", []*Key{key1}, []string{"test/"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, targetsMetadata.Delegations.Keys, key1.KeyID)
	assert.Equal(t, key1, targetsMetadata.Delegations.Keys[key1.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Equal(t, Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        Role{KeyIDs: set.NewSetFromItems(key1.KeyID), Threshold: 1},
	}, targetsMetadata.Delegations.Roles[0])

	err = targetsMetadata.UpdateDelegation("test-rule", []*Key{key1, key2}, []string{"test/"}, 1)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, key1.KeyID)
	assert.Equal(t, key1, targetsMetadata.Delegations.Keys[key1.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Keys, key2.KeyID)
	assert.Equal(t, key2, targetsMetadata.Delegations.Keys[key2.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Equal(t, Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        Role{KeyIDs: set.NewSetFromItems(key1.KeyID, key2.KeyID), Threshold: 1},
	}, targetsMetadata.Delegations.Roles[0])
}

func TestReorderDelegations(t *testing.T) {
	targetsMetadata := initialTestTargetsMetadata(t)

	key1 := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)
	key2 := ssh.NewKeyFromBytes(t, targets2PubKeyBytes)

	err := targetsMetadata.AddDelegation("rule-1", []*Key{key1}, []string{"path1/"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = targetsMetadata.AddDelegation("rule-2", []*Key{key2}, []string{"path2/"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = targetsMetadata.AddDelegation("rule-3", []*Key{key1, key2}, []string{"path3/"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	tests := map[string]struct {
		ruleNames     []string
		expected      []string
		expectedError error
	}{
		"reverse order (valid input)": {
			ruleNames:     []string{"rule-3", "rule-2", "rule-1"},
			expected:      []string{"rule-3", "rule-2", "rule-1", AllowRuleName},
			expectedError: nil,
		},
		"rule not specified in new order": {
			ruleNames:     []string{"rule-3", "rule-2"},
			expectedError: ErrMissingRules,
		},
		"rule repeated in the new order": {
			ruleNames:     []string{"rule-3", "rule-2", "rule-1", "rule-3"},
			expectedError: ErrDuplicatedRuleName,
		},
		"unknown rule in the new order": {
			ruleNames:     []string{"rule-3", "rule-2", "rule-1", "rule-4"},
			expectedError: ErrRuleNotFound,
		},
		"unknown rule in the new order (with correct length)": {
			ruleNames:     []string{"rule-3", "rule-2", "rule-4"},
			expectedError: ErrRuleNotFound,
		},
		"allow rule appears in the new order": {
			ruleNames:     []string{"rule-2", "rule-3", "rule-1", AllowRuleName},
			expectedError: ErrCannotManipulateAllowRule,
		},
	}

	for name, test := range tests {
		err = targetsMetadata.ReorderDelegations(test.ruleNames)
		if test.expectedError != nil {
			assert.ErrorIs(t, err, test.expectedError, fmt.Sprintf("unexpected error in test '%s'", name))
		} else {
			assert.Nil(t, err, fmt.Sprintf("unexpected error in test '%s'", name))
			assert.Equal(t, len(test.expected), len(targetsMetadata.Delegations.Roles),
				fmt.Sprintf("expected %d rules in test '%s', but got %d rules",
					len(test.expected), name, len(targetsMetadata.Delegations.Roles)))
			for i, ruleName := range test.expected {
				assert.Equal(t, ruleName, targetsMetadata.Delegations.Roles[i].Name,
					fmt.Sprintf("expected rule '%s' at index %d in test '%s', but got '%s'",
						ruleName, i, name, targetsMetadata.Delegations.Roles[i].Name))
			}
		}
	}
}

func TestRemoveDelegation(t *testing.T) {
	targetsMetadata := initialTestTargetsMetadata(t)

	key := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	err := targetsMetadata.AddDelegation("test-rule", []*Key{key}, []string{"test/"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))

	err = targetsMetadata.RemoveDelegation("test-rule")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Contains(t, targetsMetadata.Delegations.Keys, key.KeyID)
}

func TestAllowRule(t *testing.T) {
	allowRule := AllowRule()
	assert.Equal(t, AllowRuleName, allowRule.Name)
	assert.Equal(t, []string{"*"}, allowRule.Paths)
	assert.True(t, allowRule.Terminating)
	assert.Empty(t, allowRule.KeyIDs)
	assert.Equal(t, 1, allowRule.Threshold)
}
