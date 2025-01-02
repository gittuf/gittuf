// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"errors"

	"github.com/gittuf/gittuf/internal/attestations"
	"github.com/gittuf/gittuf/internal/cache"
	"github.com/gittuf/gittuf/internal/dev"
	"github.com/gittuf/gittuf/internal/policy"
	"github.com/gittuf/gittuf/internal/rsl"
)

var (
	ErrEntryNotNumbered = errors.New("one or more entries are not numbered")
)

func (r *Repository) PopulateCache() error {
	if !dev.InDevMode() {
		return dev.ErrNotInDevMode
	}

	persistent := &cache.Persistent{
		PolicyEntries:      []cache.RSLEntryIndex{},
		AttestationEntries: []cache.RSLEntryIndex{},
	}

	iterator, err := rsl.GetLatestEntry(r.r)
	if err != nil {
		return err
	}

	if iterator.GetNumber() == 0 {
		return ErrEntryNotNumbered
	}

	persistent.AddedAttestationsBeforeNumber = iterator.GetNumber()

	for {
		if iterator, isReferenceEntry := iterator.(*rsl.ReferenceEntry); isReferenceEntry {
			switch iterator.RefName {
			case policy.PolicyRef:
				persistent.InsertPolicyEntryNumber(iterator.GetNumber(), iterator.GetID())
			case attestations.Ref:
				persistent.InsertAttestationEntryNumber(iterator.GetNumber(), iterator.GetID())
			}
		}

		iterator, err = rsl.GetParentForEntry(r.r, iterator)
		if err != nil {
			if errors.Is(err, rsl.ErrRSLEntryNotFound) {
				break
			}

			return err
		}

		if iterator.GetNumber() == 0 {
			return ErrEntryNotNumbered
		}
	}

	return persistent.Commit(r.r)
}
