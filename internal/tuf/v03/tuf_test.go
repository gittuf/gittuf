// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v03

import (
	"testing"

	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	"github.com/secure-systems-lab/go-securesystemslib/signerverifier"
	"github.com/stretchr/testify/assert"
)

func TestTeam(t *testing.T) {
	keyR := ssh.NewKeyFromBytes(t, rootPubKeyBytes)
	key := NewKeyFromSSLibKey(keyR)
	person := &Person{
		PersonID:   "jane.doe",
		PublicKeys: map[string]*Key{key.KeyID: key},
	}
	team := &Team{
		TeamID:     "example-team",
		PublicKeys: []*Key{key},
		Members:    []*Person{person},
		Threshold:  1,
	}

	id := team.ID()
	assert.Equal(t, team.TeamID, id)

	keys := team.Keys()
	assert.Equal(t, []*signerverifier.SSLibKey{keyR}, keys)

	customMetadata := team.CustomMetadata()
	assert.Nil(t, customMetadata)

	members := team.GetMembers()
	assert.Equal(t, team.Members, members)

	threshold := team.GetThreshold()
	assert.Equal(t, team.Threshold, threshold)
}
