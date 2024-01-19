// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier"
	"github.com/gittuf/gittuf/internal/signerverifier/gpg"
	sslibsv "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/signerverifier"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestInitializeTargetsMetadata(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	assert.Equal(t, 1, targetsMetadata.Version)
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
}

func TestAddOrUpdateDelegation(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	key1, err := tuf.LoadKeyFromBytes(targets1PubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}
	key2, err := tuf.LoadKeyFromBytes(targets2PubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "test-rule", []*tuf.Key{key1, key2}, []string{"test/"})
	assert.Nil(t, err)
	assert.Contains(t, targetsMetadata.Delegations.Keys, key1.KeyID)
	assert.Equal(t, key1, targetsMetadata.Delegations.Keys[key1.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Keys, key2.KeyID)
	assert.Equal(t, key2, targetsMetadata.Delegations.Keys[key2.KeyID])
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Equal(t, tuf.Delegation{
		Name:        "test-rule",
		Paths:       []string{"test/"},
		Terminating: false,
		Role:        tuf.Role{KeyIDs: []string{key1.KeyID, key2.KeyID}, Threshold: 1},
	}, targetsMetadata.Delegations.Roles[0])
}

func TestRemoveDelegation(t *testing.T) {
	targetsMetadata := InitializeTargetsMetadata()

	key, err := tuf.LoadKeyFromBytes(targets1PubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	targetsMetadata, err = AddOrUpdateDelegation(targetsMetadata, "test-rule", []*tuf.Key{key}, []string{"test/"})
	if err != nil {
		t.Fatal(err)
	}
	assert.Equal(t, 2, len(targetsMetadata.Delegations.Roles))

	targetsMetadata, err = RemoveDelegation(targetsMetadata, "test-rule")
	assert.Nil(t, err)
	assert.Equal(t, 1, len(targetsMetadata.Delegations.Roles))
	assert.Contains(t, targetsMetadata.Delegations.Roles, AllowRule())
	assert.Contains(t, targetsMetadata.Delegations.Keys, key.KeyID)
}

func TestAddKeyToTargets(t *testing.T) {
	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	fulcioKey := &tuf.Key{
		KeyType: signerverifier.FulcioKeyType,
		Scheme:  signerverifier.FulcioKeyScheme,
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

func TestRemoveKeyFromTargets(t *testing.T) {
	gpgKey, err := gpg.LoadGPGKeyFromBytes(gpgPubKeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	fulcioKey := &tuf.Key{
		KeyType: signerverifier.FulcioKeyType,
		Scheme:  signerverifier.FulcioKeyScheme,
		KeyVal:  sslibsv.KeyVal{Identity: "jane.doe@example.com", Issuer: "https://github.com/login/oauth"},
		KeyID:   "jane.doe@example.com::https://github.com/login/oauth",
	}

	targetsKey, err := tuf.LoadKeyFromBytes(targets1KeyBytes)
	if err != nil {
		t.Fatal(err)
	}

	t.Run("remove single key", func(t *testing.T) {
		targetsMetadata := InitializeTargetsMetadata()

		assert.Nil(t, targetsMetadata.Delegations.Keys)

		targetsMetadata, err = AddKeyToTargets(targetsMetadata, []*tuf.Key{gpgKey})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(targetsMetadata.Delegations.Keys))
		assert.Equal(t, gpgKey, targetsMetadata.Delegations.Keys[gpgKey.KeyID])

		targetsMetadata, err = RemoveKeysFromTargets(targetsMetadata, []*tuf.Key{gpgKey})
		assert.Nil(t, err)
		assert.Equal(t, 0, len(targetsMetadata.Delegations.Keys))
	})

	t.Run("add multiple keys and remove multiple keys", func(t *testing.T) {
		targetsMetadata := InitializeTargetsMetadata()

		assert.Nil(t, targetsMetadata.Delegations.Keys)

		targetsMetadata, err = AddKeyToTargets(targetsMetadata, []*tuf.Key{gpgKey, fulcioKey, targetsKey})
		assert.Nil(t, err)
		assert.Equal(t, 3, len(targetsMetadata.Delegations.Keys))
		assert.Equal(t, gpgKey, targetsMetadata.Delegations.Keys[gpgKey.KeyID])
		assert.Equal(t, fulcioKey, targetsMetadata.Delegations.Keys[fulcioKey.KeyID])
		assert.Equal(t, targetsKey, targetsMetadata.Delegations.Keys[targetsKey.KeyID])

		targetsMetadata, err = RemoveKeysFromTargets(targetsMetadata, []*tuf.Key{gpgKey, targetsKey})
		assert.Nil(t, err)
		assert.Equal(t, 1, len(targetsMetadata.Delegations.Keys))
		assert.Equal(t, fulcioKey, targetsMetadata.Delegations.Keys[fulcioKey.KeyID])
	})
}

func TestAllowRule(t *testing.T) {
	allowRule := AllowRule()
	assert.Equal(t, AllowRuleName, allowRule.Name)
	assert.Equal(t, []string{"*"}, allowRule.Paths)
	assert.True(t, allowRule.Terminating)
	assert.Empty(t, allowRule.KeyIDs)
	assert.Equal(t, 1, allowRule.Threshold)
}
