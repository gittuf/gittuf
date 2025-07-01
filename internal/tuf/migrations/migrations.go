// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package migrations

import (
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
)

/*
	adityasaky: We should probably have an automatic migration to the _next_
	version every time so that it's easy to migrate from v01 to v0k easily using
	consecutive migration functions. However, I think building that out now may
	be overkill; I don't know if we expect a bunch of schema changes.
*/

// MigrateRootMetadataV01ToV02 converts tufv01.RootMetadata into
// tufv02.RootMetadata.
func MigrateRootMetadataV01ToV02(rootMetadata *tufv01.RootMetadata) *tufv02.RootMetadata {
	newRootMetadata := tufv02.NewRootMetadata()

	// Set same expires
	newRootMetadata.Expires = rootMetadata.Expires

	// Set repository location
	newRootMetadata.RepositoryLocation = rootMetadata.RepositoryLocation

	// Set keys
	newRootMetadata.Principals = map[string]tuf.Principal{}
	for keyID, key := range rootMetadata.Keys {
		newRootMetadata.Principals[keyID] = key
	}

	// Set roles
	newRootMetadata.Roles = map[string]tufv02.Role{}
	for roleName, role := range rootMetadata.Roles {
		newRole := tufv02.Role{
			PrincipalIDs: role.KeyIDs,
			Threshold:    role.Threshold,
		}
		newRootMetadata.Roles[roleName] = newRole
	}

	// Set app attestations support
	newRootMetadata.GitHubApps = rootMetadata.GitHubApps

	// Set global rules
	newRootMetadata.GlobalRules = rootMetadata.GlobalRules

	// Set propagations
	newRootMetadata.Propagations = rootMetadata.Propagations

	// Set hooks
	newRootMetadata.Hooks = rootMetadata.Hooks

	return newRootMetadata
}

// MigrateTargetsMetadataV01ToV02 converts tufv01.TargetsMetadata into
// tufv02.TargetsMetadata.
func MigrateTargetsMetadataV01ToV02(targetsMetadata *tufv01.TargetsMetadata) *tufv02.TargetsMetadata {
	newTargetsMetadata := tufv02.NewTargetsMetadata()

	// Set same expires
	newTargetsMetadata.Expires = targetsMetadata.Expires

	// Set delegations
	newTargetsMetadata.Delegations = &tufv02.Delegations{
		Principals: map[string]tuf.Principal{},
		Roles:      []*tufv02.Delegation{},
	}
	for keyID, key := range targetsMetadata.Delegations.Keys {
		newTargetsMetadata.Delegations.Principals[keyID] = key
	}
	for _, role := range targetsMetadata.Delegations.Roles {
		newRole := &tufv02.Delegation{
			Name:        role.Name,
			Paths:       role.Paths,
			Terminating: role.Terminating,
			Custom:      role.Custom,
			Role: tufv02.Role{
				PrincipalIDs: role.KeyIDs,
				Threshold:    role.Threshold,
			},
		}

		newTargetsMetadata.Delegations.Roles = append(newTargetsMetadata.Delegations.Roles, newRole)
	}

	return newTargetsMetadata
}
