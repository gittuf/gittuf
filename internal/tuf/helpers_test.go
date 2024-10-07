// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package tuf

import (
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
)

var (
	rootPubKeyBytes     = artifacts.SSHRSAPublicSSH
	targets1PubKeyBytes = artifacts.SSHECDSAPublicSSH
	targets2PubKeyBytes = artifacts.SSHED25519PublicSSH
)

func initialTestRootMetadata(t *testing.T) *RootMetadata {
	t.Helper()

	rootKey := ssh.NewKeyFromBytes(t, rootPubKeyBytes)
	rootMetadata := NewRootMetadata()
	rootMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	rootMetadata.AddKey(rootKey)

	rootMetadata.AddRole(RootRoleName, Role{
		KeyIDs:    set.NewSetFromItems(rootKey.KeyID),
		Threshold: 1,
	})

	return rootMetadata
}

func initialTestTargetsMetadata(t *testing.T) *TargetsMetadata {
	t.Helper()

	targetsMetadata := NewTargetsMetadata()
	targetsMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	targetsMetadata.Delegations.AddDelegation(AllowRule())
	return targetsMetadata
}
