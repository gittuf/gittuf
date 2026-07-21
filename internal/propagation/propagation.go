// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

// Package propagation implements gittuf's propagation workflow over
// gitinterface repositories.
package propagation

import (
	"errors"

	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/gitinterface"
	"github.com/gittuf/gittuf/pkg/rsl"
)

// PropagateChangesFromUpstreamRepository executes gittuf's propagation workflow
// to create a subtree of the contents of an upstream repository's reference
// into the specified reference and path in the downstream repository.
func PropagateChangesFromUpstreamRepository(downstreamRepo, upstreamRepo *gitinterface.Repository, details []tuf.PropagationDirective, sign bool) error {
	// FIXME: We assume here that downstreamRepo and upstreamRepo have their
	// gittuf refs already synced.

	for _, detail := range details {
		latestUpstreamEntry, _, err := rsl.GetLatestReferenceUpdaterEntry(upstreamRepo, rsl.ForReference(detail.GetUpstreamReference()), rsl.IsUnskipped())
		if err != nil {
			if !errors.Is(err, rsl.ErrRSLEntryNotFound) {
				return err
			}

			continue
		}

		// We want to check if propagation is necessary
		// What if it's already been propagated?

		// TODO: handle divergence from latest RSL entry for ref downstream?
		currentRefTip, err := downstreamRepo.GetReference(detail.GetDownstreamReference())
		if err != nil {
			return err // TODO: should we handle this differently?
		}

		currentTreeID, err := downstreamRepo.GetCommitTreeID(currentRefTip)
		if err != nil {
			return err // TODO: should we handle this differently?
		}

		currentPathTreeID, err := downstreamRepo.GetPathIDInTree(detail.GetDownstreamPath(), currentTreeID)
		if err != nil {
			if !errors.Is(err, gitinterface.ErrTreeDoesNotHavePath) {
				return err
			}
		}

		upstreamTreeID, err := upstreamRepo.GetCommitTreeID(latestUpstreamEntry.GetTargetID())
		if err != nil {
			return err
		}

		if !currentPathTreeID.IsZero() && currentPathTreeID.Equal(upstreamTreeID.Bytes()) {
			// Nothing to do
			continue
		}

		commitID, err := downstreamRepo.CreateSubtreeFromUpstreamRepository(upstreamRepo, latestUpstreamEntry.GetTargetID(), detail.GetUpstreamPath(), detail.GetDownstreamReference(), detail.GetDownstreamPath())
		if err != nil {
			return err
		}

		if err := rsl.NewPropagationEntry(detail.GetDownstreamReference(), commitID, detail.GetUpstreamRepository(), latestUpstreamEntry.GetID()).Commit(downstreamRepo, sign); err != nil {
			return err
		}

		// TODO: error management should revert propagation entries?
		// atomicity?
	}

	return nil
}
