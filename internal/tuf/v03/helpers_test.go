// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package v03

import (
	"testing"
	"time"

	"github.com/gittuf/gittuf/internal/common/set"
	"github.com/gittuf/gittuf/internal/signerverifier/ssh"
	artifacts "github.com/gittuf/gittuf/internal/testartifacts"
	"github.com/gittuf/gittuf/internal/tuf"
)

var (
	rootPubKeyBytes     = artifacts.SSHRSAPublicSSH
	targets1PubKeyBytes = artifacts.SSHECDSAPublicSSH
	targets2PubKeyBytes = artifacts.SSHED25519PublicSSH
)

func initialTestRootMetadata(t *testing.T) *RootMetadata {
	t.Helper()

	rootKey := NewKeyFromSSLibKey(ssh.NewKeyFromBytes(t, rootPubKeyBytes))
	rootMetadata := NewRootMetadata()
	rootMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	if err := rootMetadata.addPrincipal(rootKey); err != nil {
		t.Fatal(err)
	}

	rootMetadata.addRole(tuf.RootRoleName, Role{
		PrincipalIDs: set.NewSetFromItems(rootKey.KeyID),
		Threshold:    1,
	})

	return rootMetadata
}

func initialTestTargetsMetadata(t *testing.T) *TargetsMetadata {
	t.Helper()

	targetsMetadata := NewTargetsMetadata()
	targetsMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	targetsMetadata.Delegations = &Delegations{Roles: []*Delegation{AllowRule()}}
	return targetsMetadata
}
