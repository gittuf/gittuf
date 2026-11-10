// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tui

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/charmbracelet/bubbles/list"
	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/policy"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestPolicyPrincipalsFormSubmit(t *testing.T) {
	t.Run("custom metadata value containing '='", func(t *testing.T) {
		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))
		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false))

		m := &model{
			ctx: t.Context(),
			options: &options{
				p:          &persistent.Options{SigningKey: keyPath},
				policyName: policy.TargetsRoleName,
			},
			policyPrincipalsScreen: policyPrincipalsScreen{
				list: newMenuList("Policy Principals", []list.Item{}, newDelegate(4)),
			},
		}

		f := &m.policyPrincipalsFormScreen
		f.initInputs("Add Person")
		f.inputs[0].SetValue("jane.doe@example.com")
		f.inputs[1].SetValue(keyPath + ".pub")
		f.inputs[3].SetValue("profile=https://example.com/u?ref=1")
		f.focusIndex = len(f.inputs) - 1

		f.handleFormSubmit(m)

		assert.Empty(t, m.errorMsg)
		assert.Nil(t, m.errorDialog)
		assert.Equal(t, "Principal added successfully!", m.footer)
	})
}

func TestPolicyPrincipalsFormEditPerson(t *testing.T) {
	setup := func(t *testing.T) (*model, *gittuf.Repository, string, string) {
		t.Helper()

		tmpDir := t.TempDir()
		t.Chdir(tmpDir)

		gitinterface.CreateTestGitRepository(t, tmpDir, false)

		keyPath := filepath.Join(tmpDir, "test-key")
		require.NoError(t, os.WriteFile(keyPath, artifacts.SSHED25519Private, 0o600))
		require.NoError(t, os.WriteFile(keyPath+".pub", artifacts.SSHED25519PublicSSH, 0o600))

		otherKeyPath := filepath.Join(tmpDir, "other-key.pub")
		require.NoError(t, os.WriteFile(otherKeyPath, artifacts.SSHRSAPublicSSH, 0o600))

		repo, err := gittuf.LoadRepository(".")
		require.NoError(t, err)
		signer, err := gittuf.LoadSigner(repo, keyPath)
		require.NoError(t, err)
		require.NoError(t, repo.InitializeRoot(t.Context(), signer, false))
		require.NoError(t, repo.InitializeTargets(t.Context(), signer, policy.TargetsRoleName, false))

		m := &model{
			ctx: t.Context(),
			options: &options{
				p:          &persistent.Options{SigningKey: keyPath},
				policyName: policy.TargetsRoleName,
			},
			policyPrincipalsScreen: policyPrincipalsScreen{
				list: newMenuList("Policy Principals", []list.Item{}, newDelegate(4)),
			},
		}

		return m, repo, keyPath + ".pub", otherKeyPath
	}

	loadKey := func(t *testing.T, path string) *tufv02.Key {
		t.Helper()

		key, err := gittuf.LoadPublicKey(path)
		require.NoError(t, err)
		return key.(*tufv02.Key)
	}

	newPerson := func(keys ...*tufv02.Key) *tufv02.Person {
		publicKeys := map[string]*tufv02.Key{}
		for _, key := range keys {
			publicKeys[key.KeyID] = key
		}
		return &tufv02.Person{
			PersonID:             "jane.doe@example.com",
			PublicKeys:           publicKeys,
			AssociatedIdentities: map[string]string{"github": "janedoe"},
		}
	}

	submitEdit := func(t *testing.T, m *model, repo *gittuf.Repository, person *tufv02.Person, keysInput string) []string {
		t.Helper()

		f := &m.policyPrincipalsFormScreen
		f.initInputsPrefilled(person)
		if keysInput != "" {
			f.inputs[1].SetValue(keysInput)
		}
		f.inputs[3].SetValue("location=berlin")
		f.focusIndex = len(f.inputs) - 1

		f.handleFormSubmit(m)

		assert.Empty(t, m.errorMsg)
		assert.Nil(t, m.errorDialog)
		assert.Equal(t, "Principal updated successfully!", m.footer)

		principals, err := repo.ListPrincipals(t.Context(), policy.PolicyStagingRef, policy.TargetsRoleName)
		require.NoError(t, err)
		updated := principals[person.PersonID]
		require.NotNil(t, updated)
		assert.Equal(t, "berlin", updated.CustomMetadata()["location"])

		keyIDs := []string{}
		for _, key := range updated.Keys() {
			keyIDs = append(keyIDs, key.KeyID)
		}
		return keyIDs
	}

	t.Run("keep prefilled key IDs", func(t *testing.T) {
		m, repo, keyPath, _ := setup(t)
		key := loadKey(t, keyPath)
		person := newPerson(key)
		require.NoError(t, repoAddPrincipal(t.Context(), m.options, person))

		m.policyPrincipalsFormScreen.initInputsPrefilled(person)
		assert.Equal(t, key.KeyID, m.policyPrincipalsFormScreen.inputs[1].Value())

		keyIDs := submitEdit(t, m, repo, person, "")
		assert.Equal(t, []string{key.KeyID}, keyIDs)
	})

	t.Run("remove a prefilled key ID", func(t *testing.T) {
		m, repo, keyPath, otherKeyPath := setup(t)
		key := loadKey(t, keyPath)
		otherKey := loadKey(t, otherKeyPath)
		person := newPerson(key, otherKey)
		require.NoError(t, repoAddPrincipal(t.Context(), m.options, person))

		keyIDs := submitEdit(t, m, repo, person, key.KeyID)
		assert.Equal(t, []string{key.KeyID}, keyIDs)
	})

	t.Run("keep a prefilled key ID and add a new key", func(t *testing.T) {
		m, repo, keyPath, otherKeyPath := setup(t)
		key := loadKey(t, keyPath)
		otherKey := loadKey(t, otherKeyPath)
		person := newPerson(key)
		require.NoError(t, repoAddPrincipal(t.Context(), m.options, person))

		keyIDs := submitEdit(t, m, repo, person, key.KeyID+", "+otherKeyPath)
		assert.ElementsMatch(t, []string{key.KeyID, otherKey.KeyID}, keyIDs)
	})
}
