// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"github.com/gittuf/gittuf/internal/cache"
)

const (
	// gitConfigCacheEnabled is the git config key used to control
	// whether the persistent cache is enabled for this repository.
	// Users can set gittuf.cache=false to opt out.
	gitConfigCacheEnabled = "gittuf.cache"
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

// EnableCacheConfig sets gittuf.cache=true in the local git config.
// This is called automatically after clone and trust init so the
// cache is on by default without the user doing anything.
func (r *Repository) EnableCacheConfig() error {
	return r.r.SetGitConfig(gitConfigCacheEnabled, "true")
}

// IsCacheEnabled checks the local git config to see if the persistent
// cache is enabled. Returns true by default unless the user has
// explicitly set gittuf.cache=false to opt out.
func (r *Repository) IsCacheEnabled() bool {
	config, err := r.r.GetGitConfig()
	if err != nil {
		return true
	}

	val, exists := config[gitConfigCacheEnabled]
	if !exists {
		return true
	}

	return val != "false"
}
