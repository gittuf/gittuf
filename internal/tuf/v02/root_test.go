// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sort"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/gitinterface"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRootMetadata(t *testing.T) {
	rootMetadata := NewRootMetadata()

	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	err := rootMetadata.addPrincipal(key)
	assert.Nil(t, err)
	assert.Equal(t, key, rootMetadata.Principals[key.KeyID])

	person := &Person{
		PersonID:   "jane.doe@example.com",
		PublicKeys: map[string]*Key{key.KeyID: key},
	}
	err = rootMetadata.addPrincipal(person)
	assert.Nil(t, err)
	assert.Equal(t, person, rootMetadata.Principals[person.PersonID])

	t.Run("test SetExpires", func(t *testing.T) {
		d := time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC)
		rootMetadata.SetExpires(d.Format(time.RFC3339))
		assert.Equal(t, "1995-10-26T09:00:00Z", rootMetadata.Expires)
	})

	t.Run("test addRole", func(t *testing.T) {
		rootMetadata.addRole("targets", Role{
			PrincipalIDs: set.NewSetFromItems(key.KeyID),
			Threshold:    1,
		})
		assert.True(t, rootMetadata.Roles["targets"].PrincipalIDs.Has(key.KeyID))
	})

	t.Run("test SchemaVersion", func(t *testing.T) {
		schemaVersion := rootMetadata.SchemaVersion()
		assert.Equal(t, RootVersion, schemaVersion)
	})

	t.Run("test GetPrincipals", func(t *testing.T) {
		expectedPrincipals := map[string]tuf.Principal{
			key.KeyID:       key,
			person.PersonID: person,
		}

		principals := rootMetadata.GetPrincipals()
		assert.Equal(t, expectedPrincipals, principals)
	})

	t.Run("test rootLocation", func(t *testing.T) {
		currentLocation := rootMetadata.GetRepositoryLocation()
		assert.Equal(t, "", currentLocation)

		location := "https://example.com/repository/location"
		rootMetadata.SetRepositoryLocation(location)

		currentLocation = rootMetadata.GetRepositoryLocation()
		assert.Equal(t, location, currentLocation)
	})

	t.Run("test propagation directives", func(t *testing.T) {
		directives := rootMetadata.GetPropagationDirectives()
		assert.Empty(t, directives)

		directive := &PropagationDirective{
			Name:                "test",
			UpstreamRepository:  "https://example.com/git/repository",
			UpstreamReference:   "refs/heads/main",
			DownstreamReference: "refs/heads/main",
			DownstreamPath:      "upstream/",
		}
		err = rootMetadata.AddPropagationDirective(directive)
		assert.Nil(t, err)

		directives = rootMetadata.GetPropagationDirectives()
		assert.Equal(t, 1, len(directives))
		assert.Equal(t, directive, directives[0])

		err = rootMetadata.DeletePropagationDirective("test")
		assert.Nil(t, err)

		directives = rootMetadata.GetPropagationDirectives()
		assert.Empty(t, directives)

		err = rootMetadata.DeletePropagationDirective("test")
		assert.ErrorIs(t, err, tuf.ErrPropagationDirectiveNotFound)
	})

	t.Run("test multi-repository", func(t *testing.T) {
		isController := rootMetadata.IsController()
		assert.False(t, isController)

		name := "test"
		location := "http://git.example.com/repository"
		initialRootPrincipals := []tuf.Principal{key, person}

		err := rootMetadata.AddControllerRepository(name, location, initialRootPrincipals)
		assert.Nil(t, err)

		controllerRepositories := rootMetadata.GetControllerRepositories()
		assert.Equal(t, []tuf.OtherRepository{&OtherRepository{Name: name, Location: location, InitialRootPrincipals: initialRootPrincipals}}, controllerRepositories)

		propagations := rootMetadata.GetPropagationDirectives()
		foundPolicy, foundStaging := false, false
		for _, propagation := range propagations {
			if propagation.GetName() == "gittuf-controller-test-policy" {
				foundPolicy = true
			}
			if propagation.GetName() == "gittuf-controller-test-policy-staging" {
				foundStaging = true
			}
		}
		assert.True(t, foundPolicy)
		assert.True(t, foundStaging)

		err = rootMetadata.AddNetworkRepository(name, location, initialRootPrincipals)
		assert.ErrorIs(t, err, tuf.ErrNotAControllerRepository)

		err = rootMetadata.EnableController()
		assert.Nil(t, err)

		err = rootMetadata.AddNetworkRepository(name, location, initialRootPrincipals)
		assert.Nil(t, err)

		networkRepositories := rootMetadata.GetNetworkRepositories()
		assert.Equal(t, []tuf.OtherRepository{&OtherRepository{Name: name, Location: location, InitialRootPrincipals: initialRootPrincipals}}, networkRepositories)

		err = rootMetadata.DisableController()
		assert.Nil(t, err)

		networkRepositories = rootMetadata.GetNetworkRepositories()
		assert.Nil(t, networkRepositories)
	})
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
	sslibKeyO, err := ssh.NewKeyFromFile(keyPath)
	if err != nil {
		t.Fatal(err)
	}
	sslibKey := NewKeyFromSSLibKey(sslibKeyO)

	// Create TUF root and add test key
	rootMetadata := NewRootMetadata()
	if err := rootMetadata.addPrincipal(sslibKey); err != nil {
		t.Fatal(err)
	}

	// Wrap and and sign
	ctx := context.Background()
	env, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		t.Fatal(err)
	}

	verifier, err := ssh.NewVerifierFromKey(sslibKeyO)
	if err != nil {
		t.Fatal()
	}
	signer := &ssh.Signer{
		Verifier: verifier,
		Path:     keyPath,
	}

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		t.Fatal(err)
	}
	// Unwrap and verify
	// NOTE: For the sake of testing the contained key, we unwrap before we
	// verify. Typically, in DSSE it should be the other way around.
	payload, err := env.DecodeB64Payload()
	if err != nil {
		t.Fatal(err)
	}
	rootMetadata2 := &RootMetadata{}
	if err := json.Unmarshal(payload, rootMetadata2); err != nil {
		t.Log(string(payload))
		t.Fatal(err)
	}

	sslibKey2 := rootMetadata2.Principals[sslibKey.KeyID]

	// NOTE: Typically, a caller would choose this method, if KeyType==ssh.SSHKeyType
	verifier2, err := ssh.NewVerifierFromKey(sslibKey2.Keys()[0])
	if err != nil {
		t.Fatal(err)
	}
	_, err = dsse.VerifyEnvelope(ctx, env, []sslibdsse.Verifier{verifier2}, 1)
	if err != nil {
		t.Fatal(err)
	}
}

func TestAddRootPrincipal(t *testing.T) {
	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))

	t.Run("with root role already in metadata", func(t *testing.T) {
		rootMetadata := initialTestRootMetadata(t)

		newRootKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

		err := rootMetadata.AddRootPrincipal(newRootKey)
		assert.Nil(t, err)
		assert.Equal(t, newRootKey, rootMetadata.Principals[newRootKey.KeyID])
		assert.Equal(t, set.NewSetFromItems(key.KeyID, newRootKey.KeyID), rootMetadata.Roles[tuf.RootRoleName].PrincipalIDs)
	})

	t.Run("without root role already in metadata", func(t *testing.T) {
		rootMetadata := NewRootMetadata()

		err := rootMetadata.AddRootPrincipal(key)
		assert.Nil(t, err)
		assert.Equal(t, key, rootMetadata.Principals[key.KeyID])
		assert.Equal(t, set.NewSetFromItems(key.KeyID), rootMetadata.Roles[tuf.RootRoleName].PrincipalIDs)
	})

	t.Run("with person", func(t *testing.T) {
		rootMetadata := initialTestRootMetadata(t)

		person := &Person{
			PersonID: "jane.doe@example.com",
			PublicKeys: map[string]*Key{
				key.KeyID: key,
			},
		}

		err := rootMetadata.AddRootPrincipal(person)
		assert.Nil(t, err)
		assert.Equal(t, person, rootMetadata.Principals[person.PersonID])
		assert.Equal(t, set.NewSetFromItems(person.PersonID, key.KeyID), rootMetadata.Roles[tuf.RootRoleName].PrincipalIDs)
	})
}

func TestDeleteRootPrincipal(t *testing.T) {
	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))

	rootMetadata := initialTestRootMetadata(t)

	newRootKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	person := &Person{
		PersonID: "jane.doe@example.com",
		PublicKeys: map[string]*Key{
			key.KeyID: key,
		},
	}

	err := rootMetadata.AddRootPrincipal(newRootKey)
	assert.Nil(t, err)

	err = rootMetadata.AddRootPrincipal(person)
	assert.Nil(t, err)

	err = rootMetadata.DeleteRootPrincipal(newRootKey.KeyID)
	assert.Nil(t, err)
	assert.Equal(t, newRootKey, rootMetadata.Principals[newRootKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(key.KeyID, person.PersonID), rootMetadata.Roles[tuf.RootRoleName].PrincipalIDs)

	err = rootMetadata.DeleteRootPrincipal(person.PersonID)
	assert.Nil(t, err)
	assert.Equal(t, person, rootMetadata.Principals[person.PersonID])
	assert.Equal(t, set.NewSetFromItems(key.KeyID), rootMetadata.Roles[tuf.RootRoleName].PrincipalIDs)

	err = rootMetadata.DeleteRootPrincipal(key.KeyID)
	assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)
}

func TestAddPrimaryRuleFilePrincipal(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	targetsKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	err := rootMetadata.AddPrimaryRuleFilePrincipal(nil)
	assert.ErrorIs(t, err, tuf.ErrInvalidPrincipalType)

	err = rootMetadata.AddPrimaryRuleFilePrincipal(targetsKey)
	assert.Nil(t, err)
	assert.Equal(t, targetsKey, rootMetadata.Principals[targetsKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(targetsKey.KeyID), rootMetadata.Roles[tuf.TargetsRoleName].PrincipalIDs)

	person := &Person{
		PersonID: "jane.doe@example.com",
		PublicKeys: map[string]*Key{
			targetsKey.KeyID: targetsKey,
		},
	}

	err = rootMetadata.AddPrimaryRuleFilePrincipal(person)
	assert.Nil(t, err)
	assert.Equal(t, person, rootMetadata.Principals[person.PersonID])
	assert.Equal(t, set.NewSetFromItems(targetsKey.KeyID, person.PersonID), rootMetadata.Roles[tuf.TargetsRoleName].PrincipalIDs)
}

func TestDeletePrimaryRuleFilePrincipal(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	targetsKey1 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	targetsKey2 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets2PubKeyBytes))

	err := rootMetadata.AddPrimaryRuleFilePrincipal(targetsKey1)
	assert.Nil(t, err)
	err = rootMetadata.AddPrimaryRuleFilePrincipal(targetsKey2)
	assert.Nil(t, err)

	err = rootMetadata.DeletePrimaryRuleFilePrincipal("")
	assert.ErrorIs(t, err, tuf.ErrInvalidPrincipalID)

	err = rootMetadata.DeletePrimaryRuleFilePrincipal(targetsKey1.KeyID)
	assert.Nil(t, err)
	assert.Equal(t, targetsKey1, rootMetadata.Principals[targetsKey1.KeyID])
	assert.Equal(t, targetsKey2, rootMetadata.Principals[targetsKey2.KeyID])
	targetsRole := rootMetadata.Roles[tuf.TargetsRoleName]
	assert.True(t, targetsRole.PrincipalIDs.Has(targetsKey2.KeyID))

	person := &Person{
		PersonID: "jane.doe@example.com",
		PublicKeys: map[string]*Key{
			targetsKey1.KeyID: targetsKey1,
		},
	}
	err = rootMetadata.AddPrimaryRuleFilePrincipal(person)
	assert.Nil(t, err)
	assert.True(t, rootMetadata.Roles[tuf.TargetsRoleName].PrincipalIDs.Has(person.PersonID))

	err = rootMetadata.DeletePrimaryRuleFilePrincipal(person.PersonID)
	assert.Nil(t, err)
	assert.False(t, rootMetadata.Roles[tuf.TargetsRoleName].PrincipalIDs.Has(person.PersonID))

	err = rootMetadata.DeletePrimaryRuleFilePrincipal(targetsKey2.KeyID)
	assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)
}

func TestAddGitHubAppPrincipal(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	appKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	err := rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, nil)
	assert.ErrorIs(t, err, tuf.ErrInvalidPrincipalType)

	err = rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, appKey)
	assert.Nil(t, err)
	assert.Equal(t, appKey, rootMetadata.Principals[appKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(appKey.KeyID), rootMetadata.Roles[tuf.GitHubAppRoleName].PrincipalIDs)
}

func TestDeleteGitHubAppPrincipal(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	appKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	err := rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, appKey)
	assert.Nil(t, err)

	rootMetadata.DeleteGitHubAppPrincipal(tuf.GitHubAppRoleName)
	assert.Nil(t, rootMetadata.Roles[tuf.GitHubAppRoleName].PrincipalIDs)
}

func TestEnableGitHubAppApprovals(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)
	assert.False(t, rootMetadata.GitHubApprovalsTrusted)

	rootMetadata.EnableGitHubAppApprovals()
	assert.True(t, rootMetadata.GitHubApprovalsTrusted)
}

func TestDisableGitHubAppApprovals(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)
	assert.False(t, rootMetadata.GitHubApprovalsTrusted)

	rootMetadata.EnableGitHubAppApprovals()
	assert.True(t, rootMetadata.GitHubApprovalsTrusted)

	rootMetadata.DisableGitHubAppApprovals()
	assert.False(t, rootMetadata.GitHubApprovalsTrusted)
}

func TestUpdateAndGetRootThreshold(t *testing.T) {
	rootMetadata := NewRootMetadata()

	err := rootMetadata.UpdateRootThreshold(3)
	assert.ErrorIs(t, err, tuf.ErrInvalidRootMetadata)

	threshold, err := rootMetadata.GetRootThreshold()
	assert.ErrorIs(t, err, tuf.ErrInvalidRootMetadata)
	assert.Equal(t, -1, threshold)

	key1 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	key2 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	if err := rootMetadata.AddRootPrincipal(key1); err != nil {
		t.Fatal(err)
	}
	if err := rootMetadata.AddRootPrincipal(key2); err != nil {
		t.Fatal(err)
	}

	err = rootMetadata.UpdateRootThreshold(2)
	assert.Nil(t, err)
	assert.Equal(t, 2, rootMetadata.Roles[tuf.RootRoleName].Threshold)

	threshold, err = rootMetadata.GetRootThreshold()
	assert.Nil(t, err)
	assert.Equal(t, 2, threshold)

	err = rootMetadata.UpdateRootThreshold(3)
	assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)
}

func TestUpdateAndGetPrimaryRuleFileThreshold(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	err := rootMetadata.UpdatePrimaryRuleFileThreshold(3)
	assert.ErrorIs(t, err, tuf.ErrPrimaryRuleFileInformationNotFoundInRoot)

	threshold, err := rootMetadata.GetPrimaryRuleFileThreshold()
	assert.ErrorIs(t, err, tuf.ErrPrimaryRuleFileInformationNotFoundInRoot)
	assert.Equal(t, -1, threshold)

	key1 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	key2 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets2PubKeyBytes))

	if err := rootMetadata.AddPrimaryRuleFilePrincipal(key1); err != nil {
		t.Fatal(err)
	}
	if err := rootMetadata.AddPrimaryRuleFilePrincipal(key2); err != nil {
		t.Fatal(err)
	}

	err = rootMetadata.UpdatePrimaryRuleFileThreshold(2)
	assert.Nil(t, err)
	assert.Equal(t, 2, rootMetadata.Roles[tuf.TargetsRoleName].Threshold)

	threshold, err = rootMetadata.GetPrimaryRuleFileThreshold()
	assert.Nil(t, err)
	assert.Equal(t, 2, threshold)

	err = rootMetadata.UpdatePrimaryRuleFileThreshold(3)
	assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)
}

func TestGetRootPrincipals(t *testing.T) {
	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	person := &Person{
		PersonID:   "jane.doe@example.com",
		PublicKeys: map[string]*Key{key.KeyID: key},
	}

	t.Run("root role exists", func(t *testing.T) {
		rootMetadata := initialTestRootMetadata(t)

		expectedPrincipals := []tuf.Principal{key}
		rootPrincipals, err := rootMetadata.GetRootPrincipals()
		assert.Nil(t, err)
		assert.Equal(t, expectedPrincipals, rootPrincipals)
	})

	t.Run("root role does not exist", func(t *testing.T) {
		rootMetadata := NewRootMetadata()

		rootPrincipals, err := rootMetadata.GetRootPrincipals()
		assert.ErrorIs(t, err, tuf.ErrInvalidRootMetadata)
		assert.Nil(t, rootPrincipals)
	})

	t.Run("with person", func(t *testing.T) {
		rootMetadata := initialTestRootMetadata(t)

		err := rootMetadata.AddRootPrincipal(person)
		assert.Nil(t, err)

		expectedPrincipals := []tuf.Principal{key, person}
		sort.Slice(expectedPrincipals, func(i, j int) bool {
			return expectedPrincipals[i].ID() < expectedPrincipals[j].ID()
		})

		rootPrincipals, err := rootMetadata.GetRootPrincipals()
		assert.Nil(t, err)
		sort.Slice(rootPrincipals, func(i, j int) bool {
			return rootPrincipals[i].ID() < rootPrincipals[j].ID()
		})
		assert.Equal(t, expectedPrincipals, rootPrincipals)
	})
}

func TestGetPrimaryRuleFilePrincipals(t *testing.T) {
	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	person := &Person{
		PersonID:   "jane.doe@example.com",
		PublicKeys: map[string]*Key{key.KeyID: key},
	}

	t.Run("targets role exists", func(t *testing.T) {
		rootMetadata := initialTestRootMetadata(t)
		err := rootMetadata.AddPrimaryRuleFilePrincipal(key)
		assert.Nil(t, err)

		expectedPrincipals := []tuf.Principal{key}
		principals, err := rootMetadata.GetPrimaryRuleFilePrincipals()
		assert.Nil(t, err)
		assert.Equal(t, expectedPrincipals, principals)
	})

	t.Run("targets role does not exist", func(t *testing.T) {
		rootMetadata := NewRootMetadata()

		rootPrincipals, err := rootMetadata.GetPrimaryRuleFilePrincipals()
		assert.ErrorIs(t, err, tuf.ErrPrimaryRuleFileInformationNotFoundInRoot)
		assert.Nil(t, rootPrincipals)
	})

	t.Run("with person", func(t *testing.T) {
		rootMetadata := initialTestRootMetadata(t)

		err := rootMetadata.AddPrimaryRuleFilePrincipal(person)
		assert.Nil(t, err)

		expectedPrincipals := []tuf.Principal{person}
		principals, err := rootMetadata.GetPrimaryRuleFilePrincipals()
		assert.Nil(t, err)
		assert.Equal(t, expectedPrincipals, principals)
	})
}

func TestGetGitHubAppPrincipals(t *testing.T) {
	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))

	t.Run("role exists", func(t *testing.T) {
		rootMetadata := initialTestRootMetadata(t)
		err := rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, key)
		assert.Nil(t, err)

		expectedPrincipals := []tuf.Principal{key}
		principals, err := rootMetadata.GetGitHubAppPrincipals()
		assert.Nil(t, err)
		assert.Equal(t, expectedPrincipals, principals)
	})

	t.Run("role does not exist", func(t *testing.T) {
		rootMetadata := NewRootMetadata()

		rootPrincipals, err := rootMetadata.GetGitHubAppPrincipals()
		assert.ErrorIs(t, err, tuf.ErrGitHubAppInformationNotFoundInRoot)
		assert.Nil(t, rootPrincipals)
	})
}

func TestIsGitHubAppApprovalTrusted(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	trusted := rootMetadata.IsGitHubAppApprovalTrusted()
	assert.False(t, trusted)

	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	err := rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, key)
	assert.Nil(t, err)

	rootMetadata.EnableGitHubAppApprovals()
	trusted = rootMetadata.IsGitHubAppApprovalTrusted()
	assert.True(t, trusted)
}

func TestGlobalRules(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	assert.Nil(t, rootMetadata.GlobalRules) // no global rule yet

	err := rootMetadata.AddGlobalRule(NewGlobalRuleThreshold("threshold-2-main", []string{"git:refs/heads/main"}, 2))
	assert.Nil(t, err)
	err = rootMetadata.AddGlobalRule(NewGlobalRuleThreshold("threshold-2-main", []string{"git:refs/heads/main"}, 2))
	assert.ErrorIs(t, err, tuf.ErrGlobalRuleAlreadyExists)

	assert.Equal(t, 1, len(rootMetadata.GlobalRules))
	assert.Equal(t, "threshold-2-main", rootMetadata.GlobalRules[0].GetName())

	expectedGlobalRule := &GlobalRuleThreshold{
		Name:      "threshold-2-main",
		Paths:     []string{"git:refs/heads/main"},
		Threshold: 2,
	}
	globalRules := rootMetadata.GetGlobalRules()
	assert.Equal(t, expectedGlobalRule.GetName(), globalRules[0].GetName())
	assert.Equal(t, expectedGlobalRule.GetProtectedNamespaces(), globalRules[0].(tuf.GlobalRuleThreshold).GetProtectedNamespaces())
	assert.Equal(t, expectedGlobalRule.GetThreshold(), globalRules[0].(tuf.GlobalRuleThreshold).GetThreshold())

	forcePushesGlobalRule, err := NewGlobalRuleBlockForcePushes("block-force-pushes", []string{"git:refs/heads/main"})
	if err != nil {
		t.Fatal(err)
	}
	err = rootMetadata.AddGlobalRule(forcePushesGlobalRule)
	assert.Nil(t, err)
	err = rootMetadata.AddGlobalRule(forcePushesGlobalRule)
	assert.ErrorIs(t, err, tuf.ErrGlobalRuleAlreadyExists)

	assert.Equal(t, 2, len(rootMetadata.GlobalRules))
	assert.Equal(t, "threshold-2-main", rootMetadata.GlobalRules[0].GetName())
	assert.Equal(t, "block-force-pushes", rootMetadata.GlobalRules[1].GetName())

	updatedThresholdGlobalRule := &GlobalRuleThreshold{
		Name:      "threshold-2-main",
		Paths:     []string{"git:refs/heads/main"},
		Threshold: 3,
	}
	err = rootMetadata.UpdateGlobalRule(updatedThresholdGlobalRule)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(rootMetadata.GlobalRules))
	assert.Equal(t, "threshold-2-main", rootMetadata.GlobalRules[0].GetName())
	assert.Equal(t, "block-force-pushes", rootMetadata.GlobalRules[1].GetName())

	updatedForcePushesGlobalRule, err := NewGlobalRuleBlockForcePushes("block-force-pushes", []string{"git:refs/heads/*"})
	if err != nil {
		t.Fatal(err)
	}
	err = rootMetadata.UpdateGlobalRule(updatedForcePushesGlobalRule)
	assert.Nil(t, err)

	assert.Equal(t, 2, len(rootMetadata.GlobalRules))
	assert.Equal(t, "threshold-2-main", rootMetadata.GlobalRules[0].GetName())
	assert.Equal(t, "block-force-pushes", rootMetadata.GlobalRules[1].GetName())

	differentNameGlobalRule := &GlobalRuleThreshold{
		Name:      "threshold-4-main",
		Paths:     []string{"git:refs/heads/main"},
		Threshold: 4,
	}
	err = rootMetadata.UpdateGlobalRule(differentNameGlobalRule)
	assert.ErrorIs(t, err, tuf.ErrGlobalRuleNotFound)
	assert.Equal(t, 2, len(rootMetadata.GlobalRules))
	assert.Equal(t, "threshold-2-main", rootMetadata.GlobalRules[0].GetName())
	assert.Equal(t, "block-force-pushes", rootMetadata.GlobalRules[1].GetName())

	err = rootMetadata.DeleteGlobalRule("threshold-2-main")
	assert.Nil(t, err)
	err = rootMetadata.DeleteGlobalRule("block-force-pushes")
	assert.Nil(t, err)
	assert.Equal(t, 0, len(rootMetadata.GlobalRules))

	err = rootMetadata.DeleteGlobalRule("")
	assert.ErrorIs(t, err, tuf.ErrGlobalRuleNotFound)
}

func TestAddHookAndGetHooks(t *testing.T) {
	const invalidStage = 9999

	rootMetadata := initialTestRootMetadata(t)

	key1 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))
	key2 := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets2PubKeyBytes))

	_, err := rootMetadata.GetHooks(tuf.HookStagePreCommit)
	assert.ErrorIs(t, err, tuf.ErrNoHooksDefined)

	_, err = rootMetadata.AddHook([]tuf.HookStage{tuf.HookStagePreCommit}, "test-hook", []string{key1.KeyID, key2.KeyID}, map[string]string{"sha1": gitinterface.ZeroHash.String()}, tuf.HookEnvironmentLua, []string{})
	assert.Nil(t, err)

	_, err = rootMetadata.AddHook([]tuf.HookStage{tuf.HookStagePrePush}, "test-hook", []string{key1.KeyID, key2.KeyID}, map[string]string{"sha1": gitinterface.ZeroHash.String()}, tuf.HookEnvironmentLua, []string{})
	assert.Nil(t, err)

	_, err = rootMetadata.AddHook([]tuf.HookStage{tuf.HookStagePrePush}, "test-hook", []string{key1.KeyID, key2.KeyID}, map[string]string{"sha1": gitinterface.ZeroHash.String()}, tuf.HookEnvironmentLua, []string{})
	assert.ErrorIs(t, err, tuf.ErrDuplicatedHookName)

	_, err = rootMetadata.AddHook([]tuf.HookStage{invalidStage}, "test-hook", []string{key1.KeyID, key2.KeyID}, map[string]string{"sha1": gitinterface.ZeroHash.String()}, tuf.HookEnvironmentLua, []string{})
	assert.ErrorIs(t, err, tuf.ErrInvalidHookStage)

	preCommitHook := []*Hook{{
		Name:         "test-hook",
		PrincipalIDs: set.NewSetFromItems(key1.KeyID, key2.KeyID),
		Hashes:       map[string]string{"sha1": gitinterface.ZeroHash.String()},
		Environment:  tuf.HookEnvironmentLua,
		Modules:      []string{},
	}}
	assert.Equal(t, preCommitHook, rootMetadata.Hooks[tuf.HookStagePreCommit])

	prePushHook := []*Hook{{
		Name:         "test-hook",
		PrincipalIDs: set.NewSetFromItems(key1.KeyID, key2.KeyID),
		Hashes:       map[string]string{"sha1": gitinterface.ZeroHash.String()},
		Environment:  tuf.HookEnvironmentLua,
		Modules:      []string{},
	}}
	assert.Equal(t, prePushHook, rootMetadata.Hooks[tuf.HookStagePrePush])

	preCommitHooks, err := rootMetadata.GetHooks(tuf.HookStagePreCommit)
	assert.Nil(t, err)
	assert.Equal(t, 1, len(preCommitHooks))

	prePushHooks, err := rootMetadata.GetHooks(tuf.HookStagePrePush)
	assert.Nil(t, err)
	assert.Equal(t, 1, len((prePushHooks)))
}

func TestRemoveHook(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	_, err := rootMetadata.AddHook([]tuf.HookStage{tuf.HookStagePreCommit}, "test-hook", []string{key.KeyID}, map[string]string{"sha1": gitinterface.ZeroHash.String()}, tuf.HookEnvironmentLua, []string{})
	require.Nil(t, err)
	assert.Equal(t, 1, len(rootMetadata.Hooks[tuf.HookStagePreCommit]))

	_, err = rootMetadata.AddHook([]tuf.HookStage{tuf.HookStagePrePush}, "test-hook", []string{key.KeyID}, map[string]string{"sha1": gitinterface.ZeroHash.String()}, tuf.HookEnvironmentLua, []string{})
	require.Nil(t, err)
	assert.Equal(t, 1, len(rootMetadata.Hooks[tuf.HookStagePrePush]))

	err = rootMetadata.RemoveHook([]tuf.HookStage{tuf.HookStagePreCommit}, "test-hook")
	assert.Nil(t, err)
	assert.Equal(t, 0, len(rootMetadata.Hooks[tuf.HookStagePreCommit]))

	err = rootMetadata.RemoveHook([]tuf.HookStage{tuf.HookStagePrePush}, "test-hook")
	assert.Nil(t, err)
	assert.Equal(t, 0, len(rootMetadata.Hooks[tuf.HookStagePrePush]))
}
