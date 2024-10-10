// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"fmt"
	"testing"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	"github.com/gittuf/gittuf/internal/signerverifier/sigstore"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/gittuf/gittuf/internal/tuf"
	sslibsv "github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/assert"
)

func TestInitializeTargetsMetadata(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.AllowRule())
}

func TestAddDelegation(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	key1 := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)
	key2 := ssh.NewKeyFromBytes(t, targets2PubKeyBytes)

	targetsMetadata, err := AddDelegation(targetsMetadata, "test-rule", []*tuf.Key{key1, key2}, []string{"test/"}, 1)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, key1.KeyID)
	assert.Equal(t, key1, targetsMetadata.Delegations.Keys[key1.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Keys, key2.KeyID)
	assert.Equal(t, key2, targetsMetadata.Delegations.Keys[key2.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.AllowRule())
	assert.Equal(t, tuf.Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        tuf.Role{KeyIDs: set.NewSetFromItems(key1.KeyID, key2.KeyID), Threshold: 1},
	}, targetsMetadata.Delegations.Roles[0])
}

func TestUpdateDelegation(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	key1 := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)
	key2 := ssh.NewKeyFromBytes(t, targets2PubKeyBytes)

	targetsMetadata, err := AddDelegation(targetsMetadata, "test-rule", []*tuf.Key{key1}, []string{"test/"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Contains(t, targetsMetadata.Delegations.Keys, key1.KeyID)
	assert.Equal(t, key1, targetsMetadata.Delegations.Keys[key1.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.AllowRule())
	assert.Equal(t, tuf.Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        tuf.Role{KeyIDs: set.NewSetFromItems(key1.KeyID), Threshold: 1},
	}, targetsMetadata.Delegations.Roles[0])

	targetsMetadata, err = UpdateDelegation(targetsMetadata, "test-rule", []*tuf.Key{key1, key2}, []string{"test/"}, 1)
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, key1.KeyID)
	assert.Equal(t, key1, targetsMetadata.Delegations.Keys[key1.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Keys, key2.KeyID)
	assert.Equal(t, key2, targetsMetadata.Delegations.Keys[key2.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.AllowRule())
	assert.Equal(t, tuf.Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        tuf.Role{KeyIDs: set.NewSetFromItems(key1.KeyID, key2.KeyID), Threshold: 1},
	}, targetsMetadata.Delegations.Roles[0])
}

func TestReorderDelegations(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	key1 := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)
	key2 := ssh.NewKeyFromBytes(t, targets2PubKeyBytes)

	targetsMetadata, err := AddDelegation(targetsMetadata, "rule-1", []*tuf.Key{key1}, []string{"path1/"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = AddDelegation(targetsMetadata, "rule-2", []*tuf.Key{key2}, []string{"path2/"}, 1)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = AddDelegation(targetsMetadata, "rule-3", []*tuf.Key{key1, key2}, []string{"path3/"}, 1)
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
			ruleNames:     []string{"rule-2", "rule-3", "rule-1", AllowRuleName},
			expectedError: tuf.ErrCannotManipulateAllowRule,
		},
	}

	for name, test := range tests {
		_, err = ReorderDelegations(targetsMetadata, test.ruleNames)
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
	targetsMetadata := InitializeTargetsMetadata()

	key := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	targetsMetadata, err := AddDelegation(targetsMetadata, "test-rule", []*tuf.Key{key}, []string{"test/"}, 1)
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))

	targetsMetadata, err = RemoveDelegation(targetsMetadata, "test-rule")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, tuf.AllowRule())
	assert.Contains(t, targetsMetadata.Delegations.Keys, key.KeyID)
}

func TestAddKeyToTargets(t *testing.T) {
	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	fulcioKey := &tuf.Key{
		KeyType: sigstore.KeyType,
		Scheme:  sigstore.KeyScheme,
		KeyVal:  sslibsv.KeyVal{Identity: "jane.doe@example.com", Issuer: "https://github.com/login/oauth"},
		KeyID:   "jane.doe@example.com::https://github.com/login/oauth",
	}

	t.Run("add single key", func(t *testing.T) {
		targetsMetadata := InitializeTargetsMetadata()

		assert.Nil(t, targetsMetadata.Delegations.Keys)

		targetsMetadata, err = AddKeyToTargets(targetsMetadata, []*tuf.Key{gpgKey})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(targetsMetadata.Delegations.Keys))
		assert.Equal(t, gpgKey, targetsMetadata.Delegations.Keys[gpgKey.KeyID])
	})

	t.Run("add multiple keys", func(t *testing.T) {
		targetsMetadata := InitializeTargetsMetadata()

		assert.Nil(t, targetsMetadata.Delegations.Keys)

		targetsMetadata, err = AddKeyToTargets(targetsMetadata, []*tuf.Key{gpgKey, fulcioKey})
		assert.Nil(t, err)
		assert.Equal(t, 2, len(targetsMetadata.Delegations.Keys))
		assert.Equal(t, gpgKey, targetsMetadata.Delegations.Keys[gpgKey.KeyID])
		assert.Equal(t, fulcioKey, targetsMetadata.Delegations.Keys[fulcioKey.KeyID])
	})
}
