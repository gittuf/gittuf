// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"github.com/gittuf/gittuf/internal/cache"
	"github.com/gittuf/gittuf/internal/dev"
)

// PopulateCache scans the repository's RSL and generates a persistent
// local-only cache of policy and attestation entries. This makes subsequent
// verifications faster. This is currently only available in gittuf's developer
// mode.
func (r *Repository) PopulateCache() error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	return cache.PopulatePersistentCache(r.r)
}
