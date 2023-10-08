// SPDX-License-Identifier: Apache-2.0

package tuf

import (
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
