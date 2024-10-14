// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"testing"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	tuf "github.com/gittuf/gittuf/internal/tuf"
	"github.com/stretchr/testify/assert"
)

func TestInitializeRootMetadata(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := InitializeRootMetadata(key)
	assert.Equal(t, key, rootMetadata.Keys[key.KeyID])
	assert.Equal(t, 1, rootMetadata.Roles[RootRoleName].Threshold)
	assert.Equal(t, set.NewSetFromItems(key.KeyID), rootMetadata.Roles[RootRoleName].KeyIDs)
}

func TestAddRootKey(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := InitializeRootMetadata(key)

	newRootKey := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	rootMetadata, err := AddRootKey(rootMetadata, newRootKey)
	assert.Nil(t, err)
	assert.Equal(t, newRootKey, rootMetadata.Keys[newRootKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(key.KeyID, newRootKey.KeyID), rootMetadata.Roles[RootRoleName].KeyIDs)
}

func TestRemoveRootKey(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := InitializeRootMetadata(key)

	newRootKey := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	rootMetadata, err := AddRootKey(rootMetadata, newRootKey)
	assert.Nil(t, err)

	rootMetadata, err = DeleteRootKey(rootMetadata, newRootKey.KeyID)
	assert.Nil(t, err)
	assert.Equal(t, key, rootMetadata.Keys[key.KeyID])
	assert.Equal(t, newRootKey, rootMetadata.Keys[newRootKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(key.KeyID), rootMetadata.Roles[RootRoleName].KeyIDs)

	rootMetadata, err = DeleteRootKey(rootMetadata, key.KeyID)
	assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)
	assert.Nil(t, rootMetadata)
}

func TestAddTargetsKey(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := InitializeRootMetadata(key)

	targetsKey := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	_, err := AddTargetsKey(nil, targetsKey)
	assert.ErrorIs(t, err, ErrRootMetadataNil)

	_, err = AddTargetsKey(rootMetadata, nil)
	assert.ErrorIs(t, err, tuf.ErrTargetsKeyNil)

	rootMetadata, err = AddTargetsKey(rootMetadata, targetsKey)
	assert.Nil(t, err)
	assert.Equal(t, targetsKey, rootMetadata.Keys[targetsKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(targetsKey.KeyID), rootMetadata.Roles[TargetsRoleName].KeyIDs)
}

func TestDeleteTargetsKey(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := InitializeRootMetadata(key)

	targetsKey1 := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)
	targetsKey2 := ssh.NewKeyFromBytes(t, targets2PubKeyBytes)

	rootMetadata, err := AddTargetsKey(rootMetadata, targetsKey1)
	assert.Nil(t, err)
	rootMetadata, err = AddTargetsKey(rootMetadata, targetsKey2)
	assert.Nil(t, err)

	_, err = DeleteTargetsKey(nil, targetsKey1.KeyID)
	assert.ErrorIs(t, err, ErrRootMetadataNil)

	_, err = DeleteTargetsKey(rootMetadata, "")
	assert.ErrorIs(t, err, tuf.ErrKeyIDEmpty)

	rootMetadata, err = DeleteTargetsKey(rootMetadata, targetsKey1.KeyID)
	assert.Nil(t, err)
	assert.Equal(t, targetsKey1, rootMetadata.Keys[targetsKey1.KeyID])
	assert.Equal(t, targetsKey2, rootMetadata.Keys[targetsKey2.KeyID])
	targetsRole := rootMetadata.Roles[TargetsRoleName]
	assert.True(t, targetsRole.KeyIDs.Has(targetsKey2.KeyID))

	rootMetadata, err = DeleteTargetsKey(rootMetadata, targetsKey2.KeyID)
	assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)
	assert.Nil(t, rootMetadata)
}

func TestAddGitHubAppKey(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootKeyBytes)

	rootMetadata := InitializeRootMetadata(key)

	appKey := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	_, err := AddGitHubAppKey(nil, appKey)
	assert.ErrorIs(t, err, ErrRootMetadataNil)

	_, err = AddGitHubAppKey(rootMetadata, nil)
	assert.ErrorIs(t, err, tuf.ErrGitHubAppKeyNil)

	rootMetadata, err = AddGitHubAppKey(rootMetadata, appKey)
	assert.Nil(t, err)
	assert.Equal(t, appKey, rootMetadata.Keys[appKey.KeyID])
	assert.Equal(t, set.NewSetFromItems(appKey.KeyID), rootMetadata.Roles[GitHubAppRoleName].KeyIDs)
}

func TestDeleteGitHubAppKey(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := InitializeRootMetadata(key)

	appKey := ssh.NewKeyFromBytes(t, targets1PubKeyBytes)

	rootMetadata, err := AddGitHubAppKey(rootMetadata, appKey)
	assert.Nil(t, err)

	_, err = DeleteGitHubAppKey(nil)
	assert.ErrorIs(t, err, ErrRootMetadataNil)

	rootMetadata, err = DeleteGitHubAppKey(rootMetadata)
	assert.Nil(t, err)

	assert.Nil(t, rootMetadata.Roles[GitHubAppRoleName].KeyIDs)
}

func TestEnableGitHubAppApprovals(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := InitializeRootMetadata(key)

	_, err := EnableGitHubAppApprovals(nil)
	assert.ErrorIs(t, err, ErrRootMetadataNil)

	rootMetadata, err = EnableGitHubAppApprovals(rootMetadata)
	assert.Nil(t, err)
	assert.True(t, rootMetadata.GitHubApprovalsTrusted)
}

func TestDisableGitHubAppApprovals(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := InitializeRootMetadata(key)

	_, err := DisableGitHubAppApprovals(nil)
	assert.ErrorIs(t, err, ErrRootMetadataNil)

	rootMetadata, err = DisableGitHubAppApprovals(rootMetadata)
	assert.Nil(t, err)
	assert.False(t, rootMetadata.GitHubApprovalsTrusted)
}

func TestUpdateRootThreshold(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := InitializeRootMetadata(key)

	newRootKey1 := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	newRootKey2 := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata, _ = AddRootKey(rootMetadata, newRootKey1)
	rootMetadata, _ = AddRootKey(rootMetadata, newRootKey2)

	updatedRootMetadata, err := UpdateRootThreshold(rootMetadata, 4)
	assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)
	assert.Nil(t, updatedRootMetadata)

	updatedRootMetadata, err = UpdateRootThreshold(rootMetadata, 0)
	assert.Nil(t, err)
	if assert.NotNil(t, updatedRootMetadata) {
		assert.Equal(t, 0, updatedRootMetadata.Roles[RootRoleName].Threshold)
	}
}

func TestUpdateTargetsThreshold(t *testing.T) {
	key := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata := InitializeRootMetadata(key)

	targetsKey1 := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	targetsKey2 := ssh.NewKeyFromBytes(t, rootPubKeyBytes)

	rootMetadata, err := AddTargetsKey(rootMetadata, targetsKey1)
	assert.Nil(t, err)
	rootMetadata, err = AddTargetsKey(rootMetadata, targetsKey2)
	assert.Nil(t, err)

	updatedRootMetadata, err := UpdateTargetsThreshold(rootMetadata, 4)
	assert.ErrorIs(t, err, tuf.ErrCannotMeetThreshold)
	assert.Nil(t, updatedRootMetadata)

	updatedRootMetadata, err = UpdateTargetsThreshold(rootMetadata, 0)
	assert.Nil(t, err)
	if assert.NotNil(t, updatedRootMetadata) {
		assert.Equal(t, 1, updatedRootMetadata.Roles[RootRoleName].Threshold)
	}
}
