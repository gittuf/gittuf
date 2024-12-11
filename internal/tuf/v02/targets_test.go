// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"fmt"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestTargetsMetadataAndDelegations(t *testing.T) {
	targetsMetadata := NewTargetsMetadata()

	t.Run("test SetExpires", func(t *testing.T) {
		d := time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC)
		targetsMetadata.SetExpires(d.Format(time.RFC3339))
		assert.Equal(t, "1995-10-26T09:00:00Z", targetsMetadata.Expires)
	})

	// TODO: Make this test work with tuf.Hook instead of any
	//t.Run("test Validate", func(t *testing.T) {
	//	err := targetsMetadata.Validate()
	//	assert.Nil(t, err)
	//
	//	targetsMetadata.Targets = map[string]any{"test": true}
	//	err = targetsMetadata.Validate()
	//	assert.ErrorIs(t, err, ErrTargetsNotEmpty)
	//	targetsMetadata.Targets = nil
	//})

	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	person := &Person{
		PersonID:   "jane.doe",
		PublicKeys: map[string]*Key{key.KeyID: key},
	}

	delegations := &Delegations{}

	t.Run("test addPrincipal", func(t *testing.T) {
		assert.Nil(t, delegations.Principals)

		err := delegations.addPrincipal(key)
		assert.Nil(t, err)
		assert.Equal(t, key, delegations.Principals[key.KeyID])

		err = delegations.addPrincipal(person)
		assert.Nil(t, err)
		assert.Equal(t, person, delegations.Principals[person.PersonID])
	})
}

func TestDelegation(t *testing.T) {
	t.Run("matches", func(t *testing.T) {
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
	})

	t.Run("threshold", func(t *testing.T) {
		delegation := &Delegation{}

		threshold := delegation.GetThreshold()
		assert.Equal(t, 0, threshold)

		delegation.Threshold = 1
		threshold = delegation.GetThreshold()
		assert.Equal(t, 1, threshold)
	})

	t.Run("terminating", func(t *testing.T) {
		delegation := &Delegation{}

		isTerminating := delegation.IsLastTrustedInRuleFile()
		assert.False(t, isTerminating)

		delegation.Terminating = true
		isTerminating = delegation.IsLastTrustedInRuleFile()
		assert.True(t, isTerminating)
	})

	t.Run("protected namespaces", func(t *testing.T) {
		delegation := &Delegation{
			Paths: []string{"1", "2"},
		}

		protected := delegation.GetProtectedNamespaces()
		assert.Equal(t, []string{"1", "2"}, protected)
	})

	t.Run("principal IDs", func(t *testing.T) {
		keyIDs := set.NewSetFromItems("1", "2")
		delegation := &Delegation{
			Role: Role{PrincipalIDs: keyIDs},
		}

		principalIDs := delegation.GetPrincipalIDs()
		assert.Equal(t, keyIDs, principalIDs)
	})
}

func TestAddRuleAndGetRules(t *testing.T) {
	targetsMetadata := initialTestTargetsMetadata(t)

	key1 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	key2 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets2PubKeyBytes))
	person := &Person{
		PersonID:   "jane.doe",
		PublicKeys: map[string]*Key{key1.KeyID: key1},
	}

	if err := targetsMetadata.AddPrincipal(key1); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddPrincipal(key2); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddPrincipal(person); err != nil {
		t.Fatal(err)
	}

	err := targetsMetadata.AddRule("test-rule", []string{key1.KeyID, key2.KeyID, person.PersonID}, []string{"test/"}, 1)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Principals, key1.KeyID)
	assert.Equal(t, key1, targetsMetadata.Delegations.Principals[key1.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Principals, key2.KeyID)
	assert.Equal(t, key2, targetsMetadata.Delegations.Principals[key2.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Principals, person.PersonID)
	assert.Equal(t, person, targetsMetadata.Delegations.Principals[person.PersonID])
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())

	rule := &Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        Role{PrincipalIDs: set.NewSetFromItems(key1.KeyID, key2.KeyID, person.PersonID), Threshold: 1},
	}
	assert.Equal(t, rule, targetsMetadata.Delegations.Roles[0])

	rules := targetsMetadata.GetRules()
	assert.Equal(t, 2, len(rules))
	assert.Equal(t, []tuf.Rule{rule, AllowRule()}, rules)
}

func TestUpdateDelegation(t *testing.T) {
	targetsMetadata := initialTestTargetsMetadata(t)

	key1 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	key2 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets2PubKeyBytes))

	if err := targetsMetadata.AddPrincipal(key1); err != nil {
		t.Fatal(err)
	}
	err := targetsMetadata.AddRule("test-rule", []string{key1.KeyID}, []string{"test/"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, targetsMetadata.Delegations.Principals, key1.KeyID)
	assert.Equal(t, key1, targetsMetadata.Delegations.Principals[key1.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Equal(t, &Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        Role{PrincipalIDs: set.NewSetFromItems(key1.KeyID), Threshold: 1},
	}, targetsMetadata.Delegations.Roles[0])

	if err := targetsMetadata.AddPrincipal(key2); err != nil {
		t.Fatal(err)
	}
	err = targetsMetadata.UpdateRule("test-rule", []string{key1.KeyID, key2.KeyID}, []string{"test/"}, 1)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Principals, key1.KeyID)
	assert.Equal(t, key1, targetsMetadata.Delegations.Principals[key1.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Principals, key2.KeyID)
	assert.Equal(t, key2, targetsMetadata.Delegations.Principals[key2.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Equal(t, &Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        Role{PrincipalIDs: set.NewSetFromItems(key1.KeyID, key2.KeyID), Threshold: 1},
	}, targetsMetadata.Delegations.Roles[0])
}

func TestReorderRules(t *testing.T) {
	targetsMetadata := initialTestTargetsMetadata(t)

	key1 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	key2 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets2PubKeyBytes))

	if err := targetsMetadata.AddPrincipal(key1); err != nil {
		t.Fatal(err)
	}
	if err := targetsMetadata.AddPrincipal(key2); err != nil {
		t.Fatal(err)
	}

	err := targetsMetadata.AddRule("rule-1", []string{key1.KeyID}, []string{"path1/"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = targetsMetadata.AddRule("rule-2", []string{key2.KeyID}, []string{"path2/"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	err = targetsMetadata.AddRule("rule-3", []string{key1.KeyID, key2.KeyID}, []string{"path3/"}, 1)
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
			expected:      []string{"rule-3", "rule-2", "rule-1", tuf.AllowRuleName},
			expectedError: nil,
		},
		"rule not specified in new order": {
			ruleNames:     []string{"rule-3", "rule-2"},
			expectedError: tuf.ErrMissingRules,
		},
		"rule repeated in the new order": {
			ruleNames:     []string{"rule-3", "rule-2", "rule-1", "rule-3"},
			expectedError: tuf.ErrDuplicatedRuleName,
		},
		"unknown rule in the new order": {
			ruleNames:     []string{"rule-3", "rule-2", "rule-1", "rule-4"},
			expectedError: tuf.ErrRuleNotFound,
		},
		"unknown rule in the new order (with correct length)": {
			ruleNames:     []string{"rule-3", "rule-2", "rule-4"},
			expectedError: tuf.ErrRuleNotFound,
		},
		"allow rule appears in the new order": {
			ruleNames:     []string{"rule-2", "rule-3", "rule-1", tuf.AllowRuleName},
			expectedError: tuf.ErrCannotManipulateAllowRule,
		},
	}

	for name, test := range tests {
		err = targetsMetadata.ReorderRules(test.ruleNames)
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

func TestRemoveRule(t *testing.T) {
	targetsMetadata := initialTestTargetsMetadata(t)

	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	if err := targetsMetadata.AddPrincipal(key); err != nil {
		t.Fatal(err)
	}

	err := targetsMetadata.AddRule("test-rule", []string{key.KeyID}, []string{"test/"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))

	err = targetsMetadata.RemoveRule("test-rule")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Contains(t, targetsMetadata.Delegations.Principals, key.KeyID)
}

func TestGetPrincipals(t *testing.T) {
	targetsMetadata := initialTestTargetsMetadata(t)
	key1 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	if err := targetsMetadata.AddPrincipal(key1); err != nil {
		t.Fatal(err)
	}

	principals := targetsMetadata.GetPrincipals()
	assert.Equal(t, map[string]tuf.Principal{key1.KeyID: key1}, principals)

	key2 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets2PubKeyBytes))
	if err := targetsMetadata.AddPrincipal(key2); err != nil {
		t.Fatal(err)
	}

	principals = targetsMetadata.GetPrincipals()
	assert.Equal(t, map[string]tuf.Principal{key1.KeyID: key1, key2.KeyID: key2}, principals)
}

func TestAllowRule(t *testing.T) {
	allowRule := AllowRule()
	assert.Equal(t, tuf.AllowRuleName, allowRule.Name)
	assert.Equal(t, []string{"*"}, allowRule.Paths)
	assert.True(t, allowRule.Terminating)
	assert.Empty(t, allowRule.PrincipalIDs)
	assert.Equal(t, 1, allowRule.Threshold)
}
