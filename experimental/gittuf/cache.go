// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"github.com/gittuf/gittuf/internal/cache"
)

// PopulateCache scans the repository's RSL and generates a persistent
// local-only cache of policy and attestation entries. This makes subsequent
// verifications faster.
func (r *Repository) PopulateCache() error {
	return cache.PopulatePersistentCache(r.r)
}

// ResetCache deletes the local persistent cache.
func (r *Repository) ResetCache() error {
	return cache.ResetPersistentCache(r.r)
}
