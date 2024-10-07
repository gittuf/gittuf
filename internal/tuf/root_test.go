// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tuf

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/dsse"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	sslibdsse "github.com/gittuf/gittuf/internal/third_party/go-securesystemslib/dsse"
	"github.com/stretchr/testify/assert"
)

func TestRootMetadata(t *testing.T) {
	rootMetadata := NewRootMetadata()

	t.Run("test SetExpires", func(t *testing.T) {
		d := time.Date(1995, time.October, 26, 9, 0, 0, 0, time.UTC)
		rootMetadata.SetExpires(d.Format(time.RFC3339))
		assert.Equal(t, "1995-10-26T09:00:00Z", rootMetadata.Expires)
	})

	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	t.Run("test AddKey", func(t *testing.T) {
		rootMetadata.AddKey(key)
		assert.Equal(t, key, rootMetadata.Keys[key.KeyID])
	})

	t.Run("test AddRole", func(t *testing.T) {
		rootMetadata.AddRole("targets", Role{
			KeyIDs:    set.NewSetFromItems(key.KeyID),
			Threshold: 1,
		})
		assert.True(t, rootMetadata.Roles["targets"].KeyIDs.Has(key.KeyID))
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
	sslibKey, err := ssh.NewKeyFromFile(keyPath)
	if err != nil {
		t.Fatal()
	}

	// Create TUF root and add test key
	rootMetadata := NewRootMetadata()
	rootMetadata.AddKey(sslibKey)

	// Wrap and and sign
	ctx := context.Background()
	env, err := dsse.CreateEnvelope(rootMetadata)
	if err != nil {
		t.Fatal()
	}

	verifier, err := ssh.NewVerifierFromKey(sslibKey)
	if err != nil {
		t.Fatal()
	}
	signer := &ssh.Signer{
		Verifier: verifier,
		Path:     keyPath,
	}

	env, err = dsse.SignEnvelope(ctx, env, signer)
	if err != nil {
		t.Fatal()
	}
	// Unwrap and verify
	// NOTE: For the sake of testing the contained key, we unwrap before we
	// verify. Typically, in DSSE it should be the other way around.
	payload, err := env.DecodeB64Payload()
	if err != nil {
		t.Fatal()
	}
	rootMetadata2 := &RootMetadata{}
	if err := json.Unmarshal(payload, rootMetadata2); err != nil {
		t.Fatal()
	}

	sslibKey2 := rootMetadata2.Keys[sslibKey.KeyID]

	// NOTE: Typically, a caller would choose this method, if KeyType==ssh.SSHKeyType
	verifier2, err := ssh.NewVerifierFromKey(sslibKey2)
	if err != nil {
		t.Fatal()
	}
	_, err = dsse.VerifyEnvelope(ctx, env, []sslibdsse.Verifier{verifier2}, 1)
	if err != nil {
		t.Fatal()
	}
}

func TestAddRootKey(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := initialTestRootMetadata(t)

	newRootKey := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	err := rootMetadata.AddRootKey(newRootKey)
	assert.Nil(t, err)
	assert.Equal(t, newRootKey, rootMetadata.Keys[newRootKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(key.KeyID, newRootKey.KeyID), rootMetadata.Roles[RootRoleName].KeyIDs)
}

func TestDeleteRootKey(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := initialTestRootMetadata(t)

	newRootKey := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	err := rootMetadata.AddRootKey(newRootKey)
	assert.Nil(t, err)

	err = rootMetadata.DeleteRootKey(newRootKey.KeyID)
	assert.Nil(t, err)
	assert.Equal(t, key, rootMetadata.Keys[key.KeyID])
	assert.Equal(t, newRootKey, rootMetadata.Keys[newRootKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(key.KeyID), rootMetadata.Roles[RootRoleName].KeyIDs)

	err = rootMetadata.DeleteRootKey(key.KeyID)
	assert.ErrorIs(t, err, ErrCannotMeetThreshold)
}

func TestAddTargetsKey(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	targetsKey := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	err := rootMetadata.AddTargetsKey(nil)
	assert.ErrorIs(t, err, ErrTargetsKeyNil)

	err = rootMetadata.AddTargetsKey(targetsKey)
	assert.Nil(t, err)
	assert.Equal(t, targetsKey, rootMetadata.Keys[targetsKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(targetsKey.KeyID), rootMetadata.Roles[TargetsRoleName].KeyIDs)
}

func TestDeleteTargetsKey(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	targetsKey1 := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)
	targetsKey2 := ssh.NewKeyFromBytes(t, targets2PubKeyBytes)

	err := rootMetadata.AddTargetsKey(targetsKey1)
	assert.Nil(t, err)
	err = rootMetadata.AddTargetsKey(targetsKey2)
	assert.Nil(t, err)

	err = rootMetadata.DeleteTargetsKey("")
	assert.ErrorIs(t, err, ErrKeyIDEmpty)

	err = rootMetadata.DeleteTargetsKey(targetsKey1.KeyID)
	assert.Nil(t, err)
	assert.Equal(t, targetsKey1, rootMetadata.Keys[targetsKey1.KeyID])
	assert.Equal(t, targetsKey2, rootMetadata.Keys[targetsKey2.KeyID])
	targetsRole := rootMetadata.Roles[TargetsRoleName]
	assert.True(t, targetsRole.KeyIDs.Has(targetsKey2.KeyID))

	err = rootMetadata.DeleteTargetsKey(targetsKey2.KeyID)
	assert.ErrorIs(t, err, ErrCannotMeetThreshold)
}

func TestAddGitHubAppKey(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	appKey := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	err := rootMetadata.AddGitHubAppKey(nil)
	assert.ErrorIs(t, err, ErrGitHubAppKeyNil)

	err = rootMetadata.AddGitHubAppKey(appKey)
	assert.Nil(t, err)
	assert.Equal(t, appKey, rootMetadata.Keys[appKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(appKey.KeyID), rootMetadata.Roles[GitHubAppRoleName].KeyIDs)
}

func TestDeleteGitHubAppKey(t *testing.T) {
	rootMetadata := initialTestRootMetadata(t)

	appKey := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	err := rootMetadata.AddGitHubAppKey(appKey)
	assert.Nil(t, err)

	rootMetadata.DeleteGitHubAppKey()
	assert.Nil(t, rootMetadata.Roles[GitHubAppRoleName].KeyIDs)
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
