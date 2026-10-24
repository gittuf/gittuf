// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v02

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
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

	t.Run("test GetSchemaVersion", func(t *testing.T) {
		schemaVersion := rootMetadata.GetSchemaVersion()
		assert.Equal(t, RootVersion, schemaVersion)
	})

	t.Run("test GetVersion and IncrementVersion", func(t *testing.T) {
		version := rootMetadata.GetVersion()
		assert.Equal(t, uint64(1), version)

		rootMetadata.IncrementVersion()

		version = rootMetadata.GetVersion()
		assert.Equal(t, uint64(2), version)
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

		err = rootMetadata.AddPropagationDirective(directive)
		assert.ErrorIs(t, err, tuf.ErrPropagationDirectiveAlreadyExists)
		directives = rootMetadata.GetPropagationDirectives()
		assert.Equal(t, 1, len(directives))
		assert.Equal(t, directive, directives[0])

		duplicateNameDirective := &PropagationDirective{
			Name:                "test",
			UpstreamRepository:  "https://example.org/git/other-repository",
			UpstreamReference:   "refs/heads/feature",
			DownstreamReference: "refs/heads/feature",
			DownstreamPath:      "other-upstream/",
		}
		err = rootMetadata.AddPropagationDirective(duplicateNameDirective)
		assert.ErrorIs(t, err, tuf.ErrPropagationDirectiveAlreadyExists)
		directives = rootMetadata.GetPropagationDirectives()
		assert.Equal(t, 1, len(directives))
		assert.Equal(t, directive, directives[0])

		duplicateTupleDirective := &PropagationDirective{
			Name:                "other-test",
			UpstreamRepository:  "https://example.com/git/repository",
			UpstreamReference:   "refs/heads/main",
			DownstreamReference: "refs/heads/main",
			DownstreamPath:      "upstream/",
		}
		err = rootMetadata.AddPropagationDirective(duplicateTupleDirective)
		assert.ErrorIs(t, err, tuf.ErrPropagationDirectiveAlreadyExists)
		directives = rootMetadata.GetPropagationDirectives()
		assert.Equal(t, 1, len(directives))
		assert.Equal(t, directive, directives[0])

		updatedDirective := &PropagationDirective{
			Name:                "test",
			UpstreamRepository:  "https://example.org/git/repository",
			UpstreamReference:   "refs/heads/main",
			DownstreamReference: "refs/heads/main",
			DownstreamPath:      "upstream/",
		}

		err = rootMetadata.UpdatePropagationDirective(updatedDirective)
		assert.Nil(t, err)

		directives = rootMetadata.GetPropagationDirectives()
		assert.Equal(t, 1, len(directives))
		assert.Equal(t, updatedDirective, directives[0])

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

		controllers := rootMetadata.GetControllerRepositories()
		assert.Nil(t, controllers)

		networkRepositories := rootMetadata.GetNetworkRepositories()
		assert.Nil(t, networkRepositories)

		name := "test"
		location := "http://git.example.com/repository"
		initialRootPrincipals := []tuf.Principal{key, person}

		err := rootMetadata.AddControllerRepository(name, location, initialRootPrincipals)
		assert.Nil(t, err)

		// Testing duplicate controller repository detection
		// Duplicate keys
		err = rootMetadata.AddControllerRepository("test-non-duplicate", "http://git.example.com/repository-non-duplicate", initialRootPrincipals)
		assert.ErrorIs(t, err, tuf.ErrDuplicateControllerRepository)
		assert.Equal(t, 1, len(rootMetadata.MultiRepository.ControllerRepositories))

		// Duplicate names and locations
		err = rootMetadata.AddControllerRepository(name, location, []tuf.Principal{NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))})
		assert.ErrorIs(t, err, tuf.ErrDuplicateControllerRepository)
		assert.Equal(t, 1, len(rootMetadata.MultiRepository.ControllerRepositories))

		// Duplicate names
		err = rootMetadata.AddControllerRepository(name, "http://git.example.com/repository-non-duplicate", []tuf.Principal{NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))})
		assert.ErrorIs(t, err, tuf.ErrDuplicateControllerRepository)
		assert.Equal(t, 1, len(rootMetadata.MultiRepository.ControllerRepositories))

		// Duplicate locations
		err = rootMetadata.AddControllerRepository("test-non-duplicate", location, []tuf.Principal{NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))})
		assert.ErrorIs(t, err, tuf.ErrDuplicateControllerRepository)
		assert.Equal(t, 1, len(rootMetadata.MultiRepository.ControllerRepositories))

		controllerRepositories := rootMetadata.GetControllerRepositories()
		assert.Equal(t, []tuf.OtherRepository{&OtherRepository{Name: name, Location: location, InitialRootPrincipals: initialRootPrincipals}}, controllerRepositories)

		err = rootMetadata.AddNetworkRepository(name, location, initialRootPrincipals)
		assert.ErrorIs(t, err, tuf.ErrNotAControllerRepository)

		err = rootMetadata.EnableController()
		assert.Nil(t, err)

		isController = rootMetadata.IsController()
		assert.True(t, isController)

		err = rootMetadata.AddNetworkRepository(name, location, initialRootPrincipals)
		assert.Nil(t, err)

		// Testing duplicate network repository detection
		// Duplicate keys
		err = rootMetadata.AddNetworkRepository("test-non-duplicate", "http://git.example.com/repository-non-duplicate", initialRootPrincipals)
		assert.ErrorIs(t, err, tuf.ErrDuplicateNetworkRepository)
		assert.Equal(t, 1, len(rootMetadata.MultiRepository.NetworkRepositories))

		// Duplicate names and locations
		err = rootMetadata.AddNetworkRepository(name, location, []tuf.Principal{NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))})
		assert.ErrorIs(t, err, tuf.ErrDuplicateNetworkRepository)
		assert.Equal(t, 1, len(rootMetadata.MultiRepository.NetworkRepositories))

		// Duplicate names
		err = rootMetadata.AddNetworkRepository(name, "http://git.example.com/repository-non-duplicate", []tuf.Principal{NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))})
		assert.ErrorIs(t, err, tuf.ErrDuplicateNetworkRepository)
		assert.Equal(t, 1, len(rootMetadata.MultiRepository.NetworkRepositories))

		// Duplicate locations
		err = rootMetadata.AddNetworkRepository("test-non-duplicate", location, []tuf.Principal{NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))})
		assert.ErrorIs(t, err, tuf.ErrDuplicateNetworkRepository)
		assert.Equal(t, 1, len(rootMetadata.MultiRepository.NetworkRepositories))

		networkRepositories = rootMetadata.GetNetworkRepositories()
		assert.Equal(t, []tuf.OtherRepository{&OtherRepository{Name: name, Location: location, InitialRootPrincipals: initialRootPrincipals}}, networkRepositories)

		err = rootMetadata.DisableController()
		assert.Nil(t, err)

		networkRepositories = rootMetadata.GetNetworkRepositories()
		assert.Nil(t, networkRepositories)
	})

	t.Run("duplicate controller and network repository detection", func(t *testing.T) {
		alice := &Person{PersonID: "alice@example.com"}
		bob := &Person{PersonID: "bob@example.com"}
		k1 := &Key{KeyID: "key1"}
		k2 := &Key{KeyID: "key2"}

		rootMetadata := NewRootMetadata()

		// 1. Person-only repositories: different persons succeed
		err := rootMetadata.AddControllerRepository("controller-person-one", "https://example.com/p1", []tuf.Principal{alice})
		assert.NoError(t, err)

		err = rootMetadata.AddControllerRepository("controller-person-two", "https://example.com/p2", []tuf.Principal{bob})
		assert.NoError(t, err)

		// Duplicate Person-only repository fails
		err = rootMetadata.AddControllerRepository("controller-person-dup", "https://example.com/p3", []tuf.Principal{alice})
		assert.ErrorIs(t, err, tuf.ErrDuplicateControllerRepository)

		// 2. Key-only repositories: different keys succeed
		err = rootMetadata.AddControllerRepository("controller-key-one", "https://example.com/k1", []tuf.Principal{k1})
		assert.NoError(t, err)

		err = rootMetadata.AddControllerRepository("controller-key-two", "https://example.com/k2", []tuf.Principal{k2})
		assert.NoError(t, err)

		// Duplicate Key-only repository fails
		err = rootMetadata.AddControllerRepository("controller-key-dup", "https://example.com/k3", []tuf.Principal{k1})
		assert.ErrorIs(t, err, tuf.ErrDuplicateControllerRepository)

		// 3. Mixed repositories: {k1, alice} vs {k1, bob} succeed
		err = rootMetadata.AddControllerRepository("controller-mixed-one", "https://example.com/m1", []tuf.Principal{k1, alice})
		assert.NoError(t, err)

		err = rootMetadata.AddControllerRepository("controller-mixed-two", "https://example.com/m2", []tuf.Principal{k1, bob})
		assert.NoError(t, err)

		// Duplicate mixed repository fails
		err = rootMetadata.AddControllerRepository("controller-mixed-dup", "https://example.com/m3", []tuf.Principal{k1, alice})
		assert.ErrorIs(t, err, tuf.ErrDuplicateControllerRepository)

		// Network repositories
		err = rootMetadata.EnableController()
		assert.NoError(t, err)

		// 1. Person-only repositories: different persons succeed
		err = rootMetadata.AddNetworkRepository("network-person-one", "https://example.com/p1", []tuf.Principal{alice})
		assert.NoError(t, err)

		err = rootMetadata.AddNetworkRepository("network-person-two", "https://example.com/p2", []tuf.Principal{bob})
		assert.NoError(t, err)

		// Duplicate Person-only repository fails
		err = rootMetadata.AddNetworkRepository("network-person-dup", "https://example.com/p3", []tuf.Principal{alice})
		assert.ErrorIs(t, err, tuf.ErrDuplicateNetworkRepository)

		// 2. Key-only repositories: different keys succeed
		err = rootMetadata.AddNetworkRepository("network-key-one", "https://example.com/k1", []tuf.Principal{k1})
		assert.NoError(t, err)

		err = rootMetadata.AddNetworkRepository("network-key-two", "https://example.com/k2", []tuf.Principal{k2})
		assert.NoError(t, err)

		// Duplicate Key-only repository fails
		err = rootMetadata.AddNetworkRepository("network-key-dup", "https://example.com/k3", []tuf.Principal{k1})
		assert.ErrorIs(t, err, tuf.ErrDuplicateNetworkRepository)

		// 3. Mixed repositories: {k1, alice} vs {k1, bob} succeed
		err = rootMetadata.AddNetworkRepository("network-mixed-one", "https://example.com/m1", []tuf.Principal{k1, alice})
		assert.NoError(t, err)

		err = rootMetadata.AddNetworkRepository("network-mixed-two", "https://example.com/m2", []tuf.Principal{k1, bob})
		assert.NoError(t, err)

		// Duplicate mixed repository fails
		err = rootMetadata.AddNetworkRepository("network-mixed-dup", "https://example.com/m3", []tuf.Principal{k1, alice})
		assert.ErrorIs(t, err, tuf.ErrDuplicateNetworkRepository)
	})
}

func TestRootMetadataUnmarshalling(t *testing.T) {
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

	globalRuleThreshold := tufv01.NewGlobalRuleThreshold("gr-threshold", []string{"git:refs/heads/main"}, 1)

	if err := rootMetadata.AddGlobalRule(globalRuleThreshold); err != nil {
		t.Fatal(err)
	}

	globalRuleBlockForcePushes, err := tufv01.NewGlobalRuleBlockForcePushes("gr-blockforcepushes", []string{"git:refs/heads/main"})
	if err != nil {
		t.Fatal(err)
	}

	if err := rootMetadata.AddGlobalRule(globalRuleBlockForcePushes); err != nil {
		t.Fatal(err)
	}

	propagationDirective := NewPropagationDirective("pd", "upstream", "main", "example.com", "main", "example.com")

	if err := rootMetadata.AddPropagationDirective(propagationDirective); err != nil {
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

	t.Run("miscellaneous error checking", func(t *testing.T) {
		rootMetadata := initialTestRootMetadata(t)

		// Test nil principal check
		err := rootMetadata.AddRootPrincipal(nil)
		assert.ErrorIs(t, err, tuf.ErrInvalidPrincipalType)
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

	// SSH key IDs are case-sensitive, a lowercased ID must not match
	err = rootMetadata.DeleteRootPrincipal(strings.ToLower(newRootKey.KeyID))
	assert.ErrorIs(t, err, tuf.ErrPrincipalNotFound)
	assert.Equal(t, set.NewSetFromItems(key.KeyID, newRootKey.KeyID, person.PersonID), rootMetadata.Roles[tuf.RootRoleName].PrincipalIDs)

	err = rootMetadata.DeleteRootPrincipal(newRootKey.KeyID)
	assert.Nil(t, err)
	assert.Equal(t, newRootKey, rootMetadata.Principals[newRootKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(key.KeyID, person.PersonID), rootMetadata.Roles[tuf.RootRoleName].PrincipalIDs)

	err = rootMetadata.DeleteRootPrincipal(person.PersonID)
	assert.Nil(t, err)
	assert.Equal(t, person, rootMetadata.Principals[person.PersonID])
	assert.Equal(t, set.NewSetFromItems(key.KeyID), rootMetadata.Roles[tuf.RootRoleName].PrincipalIDs)

	err = rootMetadata.DeleteRootPrincipal("unknown")
	assert.ErrorIs(t, err, tuf.ErrPrincipalNotFound)

	err = rootMetadata.DeleteRootPrincipal(key.KeyID)
	assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		rootMetadata := NewRootMetadata()

		// Test non-existent root role check
		err := rootMetadata.DeleteRootPrincipal("")
		assert.ErrorIs(t, err, tuf.ErrInvalidRootMetadata)
	})
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

	// SSH key IDs are case-sensitive, a lowercased ID must not match
	err = rootMetadata.DeletePrimaryRuleFilePrincipal(strings.ToLower(targetsKey1.KeyID))
	assert.ErrorIs(t, err, tuf.ErrPrincipalNotFound)
	assert.Equal(t, set.NewSetFromItems(targetsKey1.KeyID, targetsKey2.KeyID), rootMetadata.Roles[tuf.TargetsRoleName].PrincipalIDs)

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

	err = rootMetadata.DeletePrimaryRuleFilePrincipal("unknown")
	assert.ErrorIs(t, err, tuf.ErrPrincipalNotFound)

	err = rootMetadata.DeletePrimaryRuleFilePrincipal(targetsKey2.KeyID)
	assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)

	t.Run("miscellaneous error checking", func(t *testing.T) {
		rootMetadata := NewRootMetadata()

		// Test non-existent primary rule file info check
		err := rootMetadata.DeletePrimaryRuleFilePrincipal("bob")
		assert.ErrorIs(t, err, tuf.ErrPrimaryRuleFileInformationNotFoundInRoot)
	})
}

func TestAddGitHubAppPrincipal(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	appKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	err := rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, nil)
	assert.ErrorIs(t, err, tuf.ErrInvalidPrincipalType)

	err = rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, appKey)
	assert.Nil(t, err)
	assert.Equal(t, appKey, rootMetadata.Principals[appKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(appKey.KeyID), rootMetadata.GitHubApps[tuf.GitHubAppRoleName].PrincipalIDs)
}

func TestDeleteGitHubAppPrincipal(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	// Cover nil check
	err := rootMetadata.DeleteGitHubAppPrincipal("")
	assert.ErrorIs(t, err, tuf.ErrGitHubAppNotFound)
	assert.Nil(t, rootMetadata.GitHubApps)

	appKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	err = rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, appKey)
	assert.Nil(t, err)

	// Non-existent app when other apps are present
	err = rootMetadata.DeleteGitHubAppPrincipal("does-not-exist")
	assert.ErrorIs(t, err, tuf.ErrGitHubAppNotFound)
	assert.Contains(t, rootMetadata.GitHubApps, tuf.GitHubAppRoleName)

	err = rootMetadata.DeleteGitHubAppPrincipal(tuf.GitHubAppRoleName)
	assert.NoError(t, err)
	assert.NotContains(t, rootMetadata.GitHubApps, tuf.GitHubAppRoleName)
}

func TestEnableGitHubAppApprovals(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	appName := "github-app"
	appKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	err := rootMetadata.AddGitHubAppPrincipal(appName, appKey)
	require.Nil(t, err)

	err = rootMetadata.EnableGitHubAppApprovals(appName)
	assert.NoError(t, err)
	assert.True(t, rootMetadata.GitHubApps[appName].Trusted)

	err = rootMetadata.EnableGitHubAppApprovals("does-not-exist")
	assert.ErrorIs(t, err, tuf.ErrGitHubAppNotFound)
	assert.NotContains(t, rootMetadata.GitHubApps, "does-not-exist")
}

func TestDisableGitHubAppApprovals(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	appName := "github-app"
	appKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	err := rootMetadata.AddGitHubAppPrincipal(appName, appKey)
	require.Nil(t, err)

	err = rootMetadata.EnableGitHubAppApprovals(appName)
	assert.NoError(t, err)
	assert.True(t, rootMetadata.GitHubApps[appName].Trusted)

	err = rootMetadata.DisableGitHubAppApprovals(appName)
	assert.NoError(t, err)
	assert.False(t, rootMetadata.GitHubApps[appName].Trusted)

	err = rootMetadata.DisableGitHubAppApprovals("does-not-exist")
	assert.ErrorIs(t, err, tuf.ErrGitHubAppNotFound)
	assert.NotContains(t, rootMetadata.GitHubApps, "does-not-exist")
}

func TestGetGitHubAppEntries(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	entries, err := rootMetadata.GetGitHubAppEntries()
	assert.Nil(t, err)
	assert.Nil(t, entries)

	appName := "github-app"
	appKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, targets1PubKeyBytes))

	err = rootMetadata.AddGitHubAppPrincipal(appName, appKey)
	require.Nil(t, err)

	entries, err = rootMetadata.GetGitHubAppEntries()
	assert.Nil(t, err)
	assert.Len(t, entries, 1)
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

	err = rootMetadata.UpdateRootThreshold(-1)
	assert.ErrorIs(t, err, tuf.ErrInvalidThreshold)
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

	err = rootMetadata.UpdatePrimaryRuleFileThreshold(-1)
	assert.ErrorIs(t, err, tuf.ErrInvalidThreshold)
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
		principals, err := rootMetadata.GetGitHubAppPrincipals(tuf.GitHubAppRoleName)
		assert.Nil(t, err)
		assert.Equal(t, expectedPrincipals, principals)
	})

	t.Run("role does not exist", func(t *testing.T) {
		rootMetadata := NewRootMetadata()

		rootPrincipals, err := rootMetadata.GetGitHubAppPrincipals(tuf.GitHubAppRoleName)
		assert.ErrorIs(t, err, tuf.ErrGitHubAppInformationNotFoundInRoot)
		assert.Nil(t, rootPrincipals)
	})
}

func TestIsGitHubAppApprovalTrusted(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	err := rootMetadata.AddGitHubAppPrincipal(tuf.GitHubAppRoleName, key)
	assert.Nil(t, err)

	err = rootMetadata.EnableGitHubAppApprovals(tuf.GitHubAppRoleName)
	assert.NoError(t, err)
	trusted := rootMetadata.IsGitHubAppApprovalTrusted(tuf.GitHubAppRoleName)
	assert.True(t, trusted)

	assert.False(t, rootMetadata.IsGitHubAppApprovalTrusted("non-existent-app"))
}

func TestGlobalRules(t *testing.T) {
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
			thresholdRule := GlobalRuleThreshold{Paths: test.patterns}
			blockForcePushesRule := GlobalRuleBlockForcePushes{Paths: test.patterns}
			got := thresholdRule.Matches(test.target)
			assert.Equal(t, test.expected, got, fmt.Sprintf("unexpected result in test '%s'", name))
			got = blockForcePushesRule.Matches(test.target)
			assert.Equal(t, test.expected, got, fmt.Sprintf("unexpected result in test '%s'", name))
		}
	})

	rootMetadata := initialTestRootMetadata(t)

	assert.Nil(t, rootMetadata.GlobalRules) // no global rule yet

	err := rootMetadata.AddGlobalRule(NewGlobalRuleThreshold("invalid-threshold", []string{"git:refs/heads/main"}, 0))
	assert.ErrorIs(t, err, tuf.ErrInvalidThreshold)
	assert.Nil(t, rootMetadata.GlobalRules)

	err = rootMetadata.AddGlobalRule(NewGlobalRuleThreshold("threshold-2-main", []string{"git:refs/heads/main"}, 2))
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

	invalidThresholdGlobalRule := &GlobalRuleThreshold{
		Name:      "threshold-2-main",
		Paths:     []string{"git:refs/heads/main"},
		Threshold: 0,
	}
	err = rootMetadata.UpdateGlobalRule(invalidThresholdGlobalRule)
	assert.ErrorIs(t, err, tuf.ErrInvalidThreshold)

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

	err = rootMetadata.DeleteGlobalRule("does-not-exist")
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

type unsupportedRootPrincipal struct {
	id string
}

func (p *unsupportedRootPrincipal) ID() string {
	return p.id
}

func (p *unsupportedRootPrincipal) Keys() []*signerverifier.SSLibKey {
	return nil
}

func (p *unsupportedRootPrincipal) CustomMetadata() map[string]string {
	return nil
}

func TestRootMetadataUnsupportedPrincipal(t *testing.T) {
	rootMetadata := NewRootMetadata()
	unsupported := &unsupportedRootPrincipal{id: "unsupported"}

	err := rootMetadata.AddRootPrincipal(unsupported)
	assert.ErrorIs(t, err, tuf.ErrInvalidPrincipalType)

	err = rootMetadata.AddPrimaryRuleFilePrincipal(unsupported)
	assert.ErrorIs(t, err, tuf.ErrInvalidPrincipalType)

	err = rootMetadata.AddGitHubAppPrincipal("app", unsupported)
	assert.ErrorIs(t, err, tuf.ErrInvalidPrincipalType)

	err = rootMetadata.AddControllerRepository("test", "loc", []tuf.Principal{unsupported})
	assert.ErrorIs(t, err, tuf.ErrInvalidPrincipalType)

	err = rootMetadata.EnableController()
	assert.Nil(t, err)

	err = rootMetadata.AddNetworkRepository("test", "loc", []tuf.Principal{unsupported})
	assert.ErrorIs(t, err, tuf.ErrInvalidPrincipalType)
}

func TestOtherRepositoryMethods(t *testing.T) {
	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	repo := &OtherRepository{
		Name:                  "repo-name",
		Location:              "repo-loc",
		InitialRootPrincipals: []tuf.Principal{key},
	}

	assert.Equal(t, "repo-name", repo.GetName())
	assert.Equal(t, "repo-loc", repo.Location)
	assert.Equal(t, "repo-loc", repo.GetLocation())
	assert.Equal(t, []tuf.Principal{key}, repo.GetInitialRootPrincipals())

	t.Run("UnmarshalJSON person and key", func(t *testing.T) {
		person := &Person{PersonID: "alice", PublicKeys: map[string]*Key{key.KeyID: key}}
		other := &OtherRepository{
			Name:                  "mixed",
			Location:              "loc",
			InitialRootPrincipals: []tuf.Principal{key, person},
		}

		data, err := json.Marshal(other)
		assert.Nil(t, err)

		got := &OtherRepository{}
		err = json.Unmarshal(data, got)
		assert.Nil(t, err)
		assert.Equal(t, other.Name, got.Name)
		assert.Equal(t, other.Location, got.Location)
		assert.Len(t, got.InitialRootPrincipals, 2)
	})

	t.Run("UnmarshalJSON errors", func(t *testing.T) {
		err := json.Unmarshal([]byte(`{"initialRootPrincipals": ["bad"]}`), &OtherRepository{})
		assert.ErrorContains(t, err, "unable to unmarshal json")

		err = json.Unmarshal([]byte(`{"initialRootPrincipals": [{"keyid": []}]}`), &OtherRepository{})
		assert.ErrorContains(t, err, "unable to unmarshal json")

		err = json.Unmarshal([]byte(`{"initialRootPrincipals": [{"personID": []}]}`), &OtherRepository{})
		assert.ErrorContains(t, err, "unable to unmarshal json")

		err = json.Unmarshal([]byte(`{"initialRootPrincipals": [{"unknown": true}]}`), &OtherRepository{})
		assert.ErrorContains(t, err, "unrecognized principal type")
	})
}

func TestRootMetadataUnmarshalJSON(t *testing.T) {
	key := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	person := &Person{PersonID: "alice", PublicKeys: map[string]*Key{key.KeyID: key}}

	rootMetadata := NewRootMetadata()
	err := rootMetadata.AddRootPrincipal(key)
	assert.Nil(t, err)
	err = rootMetadata.AddRootPrincipal(person)
	assert.Nil(t, err)

	ruleThreshold := NewGlobalRuleThreshold("gr-thresh", []string{"git:refs/heads/main"}, 1)
	err = rootMetadata.AddGlobalRule(ruleThreshold)
	assert.Nil(t, err)

	ruleForcePushes, err := NewGlobalRuleBlockForcePushes("gr-fp", []string{"git:refs/heads/main"})
	assert.Nil(t, err)
	err = rootMetadata.AddGlobalRule(ruleForcePushes)
	assert.Nil(t, err)

	pd := NewPropagationDirective("pd", "upstream", "refs/heads/main", "", "refs/heads/main", "")
	err = rootMetadata.AddPropagationDirective(pd)
	assert.Nil(t, err)

	data, err := json.Marshal(rootMetadata)
	assert.Nil(t, err)

	got := &RootMetadata{}
	err = json.Unmarshal(data, got)
	assert.Nil(t, err)
	assert.Equal(t, rootMetadata.Type, got.Type)
	assert.Equal(t, rootMetadata.SchemaVersion, got.SchemaVersion)
	assert.Len(t, got.Principals, 2)
	assert.Len(t, got.GlobalRules, 2)
	assert.Len(t, got.Propagations, 1)

	t.Run("UnmarshalJSON error cases", func(t *testing.T) {
		// Invalid principal JSON
		err := json.Unmarshal([]byte(`{"principals": {"p": "bad"}}`), &RootMetadata{})
		assert.ErrorContains(t, err, "unable to unmarshal json")

		// Malformed key principal
		err = json.Unmarshal([]byte(`{"principals": {"p": {"keyid": []}}}`), &RootMetadata{})
		assert.ErrorContains(t, err, "unable to unmarshal json")

		// Malformed person principal
		err = json.Unmarshal([]byte(`{"principals": {"p": {"personID": []}}}`), &RootMetadata{})
		assert.ErrorContains(t, err, "unable to unmarshal json")

		// Unknown principal type
		err = json.Unmarshal([]byte(`{"principals": {"p": {"unknown": true}}}`), &RootMetadata{})
		assert.ErrorContains(t, err, "unrecognized principal type")

		// Invalid global rule JSON
		err = json.Unmarshal([]byte(`{"globalRules": ["bad"]}`), &RootMetadata{})
		assert.ErrorContains(t, err, "unable to unmarshal json")

		// Malformed threshold global rule
		err = json.Unmarshal([]byte(`{"globalRules": [{"type": "threshold", "threshold": "invalid"}]}`), &RootMetadata{})
		assert.ErrorContains(t, err, "unable to unmarshal json")

		// Malformed block-force-pushes global rule
		err = json.Unmarshal([]byte(`{"globalRules": [{"type": "block-force-pushes", "paths": "invalid"}]}`), &RootMetadata{})
		assert.ErrorContains(t, err, "unable to unmarshal json")

		// Unknown global rule type
		err = json.Unmarshal([]byte(`{"globalRules": [{"type": "unknown"}]}`), &RootMetadata{})
		assert.ErrorIs(t, err, tuf.ErrUnknownGlobalRuleType)

		// Invalid propagation directive JSON
		err = json.Unmarshal([]byte(`{"propagations": ["bad"]}`), &RootMetadata{})
		assert.ErrorContains(t, err, "unable to unmarshal json for propagation directive")
	})
}

func TestRootMetadataControllerToggle(t *testing.T) {
	rootMetadata := NewRootMetadata()
	assert.Nil(t, rootMetadata.MultiRepository)

	err := rootMetadata.DisableController()
	assert.Nil(t, err)

	err = rootMetadata.EnableController()
	assert.Nil(t, err)
	assert.True(t, rootMetadata.IsController())

	err = rootMetadata.DisableController()
	assert.Nil(t, err)
	assert.False(t, rootMetadata.IsController())
}

func TestRootMetadataUpdateGlobalRuleAndPropagationErrors(t *testing.T) {
	rootMetadata := NewRootMetadata()

	// UpdateGlobalRule on empty rules
	ruleThreshold := NewGlobalRuleThreshold("gr-thresh", []string{"git:refs/heads/main"}, 1)
	err := rootMetadata.UpdateGlobalRule(ruleThreshold)
	assert.ErrorIs(t, err, tuf.ErrGlobalRuleNotFound)

	// UpdatePropagationDirective on empty propagations
	pd := NewPropagationDirective("pd", "upstream", "refs/heads/main", "", "refs/heads/main", "")
	err = rootMetadata.UpdatePropagationDirective(pd)
	assert.ErrorIs(t, err, tuf.ErrPropagationDirectiveNotFound)

	// Add rules and test type mismatch
	err = rootMetadata.AddGlobalRule(ruleThreshold)
	assert.Nil(t, err)

	ruleForcePushes, err := NewGlobalRuleBlockForcePushes("gr-thresh", []string{"git:refs/heads/main"})
	assert.Nil(t, err)
	err = rootMetadata.UpdateGlobalRule(ruleForcePushes)
	assert.ErrorIs(t, err, tuf.ErrCannotUpdateGlobalRuleType)

	// Add force pushes rule and try updating with threshold rule with same name
	ruleFP2, err := NewGlobalRuleBlockForcePushes("gr-fp", []string{"git:refs/heads/main"})
	assert.Nil(t, err)
	err = rootMetadata.AddGlobalRule(ruleFP2)
	assert.Nil(t, err)

	ruleThreshMismatch := NewGlobalRuleThreshold("gr-fp", []string{"git:refs/heads/main"}, 1)
	err = rootMetadata.UpdateGlobalRule(ruleThreshMismatch)
	assert.ErrorIs(t, err, tuf.ErrCannotUpdateGlobalRuleType)

	// Add propagation directive and test not found on non-empty list
	err = rootMetadata.AddPropagationDirective(pd)
	assert.Nil(t, err)

	pdNotFound := NewPropagationDirective("pd-non-existent", "upstream", "refs/heads/main", "", "refs/heads/main", "")
	err = rootMetadata.UpdatePropagationDirective(pdNotFound)
	assert.ErrorIs(t, err, tuf.ErrPropagationDirectiveNotFound)
}

