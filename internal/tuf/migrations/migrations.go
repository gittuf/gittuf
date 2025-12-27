// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package migrations

import (
	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	tufv03 "github.com/gittuf/gittuf/internal/tuf/v03"
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

	if rootMetadata.MultiRepository != nil {
		newRootMetadata.MultiRepository = &tufv02.MultiRepository{
			Controller:             rootMetadata.MultiRepository.Controller,
			ControllerRepositories: []*tufv02.OtherRepository{},
			NetworkRepositories:    []*tufv02.OtherRepository{},
		}

		for _, otherRepository := range rootMetadata.MultiRepository.ControllerRepositories {
			newRootMetadata.MultiRepository.ControllerRepositories = append(newRootMetadata.MultiRepository.ControllerRepositories, &tufv02.OtherRepository{
				Name:                  otherRepository.GetName(),
				Location:              otherRepository.GetLocation(),
				InitialRootPrincipals: otherRepository.GetInitialRootPrincipals(),
			})
		}

		for _, otherRepository := range rootMetadata.MultiRepository.NetworkRepositories {
			newRootMetadata.MultiRepository.NetworkRepositories = append(newRootMetadata.MultiRepository.NetworkRepositories, &tufv02.OtherRepository{
				Name:                  otherRepository.GetName(),
				Location:              otherRepository.GetLocation(),
				InitialRootPrincipals: otherRepository.GetInitialRootPrincipals(),
			})
		}
	}

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

// MigrateRootMetadataV02ToV03 converts tufv02.RootMetadata into
// tufv03.RootMetadata.
func MigrateRootMetadataV02ToV03(rootMetadata *tufv02.RootMetadata) *tufv03.RootMetadata {
	newRootMetadata := tufv03.NewRootMetadata()

	// Set same expires
	newRootMetadata.Expires = rootMetadata.Expires

	// Set repository location
	newRootMetadata.RepositoryLocation = rootMetadata.RepositoryLocation

	// Set keys
	newRootMetadata.Principals = rootMetadata.Principals

	// Set roles
	newRootMetadata.Roles = map[string]tufv03.Role{}
	for roleName, role := range rootMetadata.Roles {
		newRole := tufv03.Role{
			PrincipalIDs: role.PrincipalIDs,
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

	if rootMetadata.MultiRepository != nil {
		for _, otherRepository := range rootMetadata.MultiRepository.ControllerRepositories {
			newRootMetadata.MultiRepository.ControllerRepositories = append(newRootMetadata.MultiRepository.ControllerRepositories, &tufv03.OtherRepository{
				Name:                  otherRepository.GetName(),
				Location:              otherRepository.GetLocation(),
				InitialRootPrincipals: otherRepository.GetInitialRootPrincipals(),
			})
		}

		for _, otherRepository := range rootMetadata.MultiRepository.NetworkRepositories {
			newRootMetadata.MultiRepository.NetworkRepositories = append(newRootMetadata.MultiRepository.NetworkRepositories, &tufv03.OtherRepository{
				Name:                  otherRepository.GetName(),
				Location:              otherRepository.GetLocation(),
				InitialRootPrincipals: otherRepository.GetInitialRootPrincipals(),
			})
		}
	}

	// Set hooks
	newRootMetadata.Hooks = rootMetadata.Hooks

	return newRootMetadata
}

// MigrateTargetsMetadataV02ToV03 converts tufv02.TargetsMetadata into
// tufv03.TargetsMetadata.
func MigrateTargetsMetadataV02ToV03(targetsMetadata *tufv02.TargetsMetadata) *tufv03.TargetsMetadata {
	newTargetsMetadata := tufv03.NewTargetsMetadata()

	// Set same expires
	newTargetsMetadata.Expires = targetsMetadata.Expires

	// Set delegations
	newTargetsMetadata.Delegations = &tufv03.Delegations{
		Principals: targetsMetadata.Delegations.Principals,
		Roles:      []*tufv03.Delegation{},
	}
	for _, role := range targetsMetadata.Delegations.Roles {
		newRole := &tufv03.Delegation{
			Name:        role.Name,
			Paths:       role.Paths,
			Terminating: role.Terminating,
			Custom:      role.Custom,
			Role: tufv03.Role{
				PrincipalIDs: role.PrincipalIDs,
				Threshold:    role.Threshold,
			},
		}

		newTargetsMetadata.Delegations.Roles = append(newTargetsMetadata.Delegations.Roles, newRole)
	}

	return newTargetsMetadata
}
