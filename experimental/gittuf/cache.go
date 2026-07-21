// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"github.com/gittuf/gittuf/internal/cache"
	"github.com/gittuf/gittuf/pkg/gitstore"
)

const (
	// CacheAutomaticEnablementGitKey is the git config key used to control
	// whether gittuf automatically enables the persistent cache after clone and
	// trust init. Users can set this to "false" to opt out.
	CacheAutomaticEnablementGitKey = string(gitstore.ConfigCacheAutomatic)
)

// PopulateCache scans the repository's RSL and generates a persistent
// local-only cache of policy and attestation entries. This makes subsequent
// verifications faster.
func (r *Repository) PopulateCache() error {
	return cache.PopulatePersistentCache(r.r)
}

// DeleteCache deletes the local persistent cache.
func (r *Repository) DeleteCache() error {
	return cache.DeletePersistentCache(r.r)
}

// GetAutomaticCacheEnablementStatus returns if the user has opted out of
// automatic cache enablement. Returns false by default (meaning automatic
// enablement is enabled) unless gittuf.cache.automatic=false is set.
func (r *Repository) GetAutomaticCacheEnablementStatus() bool {
	val, ok, err := r.r.LookupConfig(gitstore.ConfigCacheAutomatic)
	if err != nil || !ok {
		return false
	}

	return val == "false"
}
