// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"errors"

	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/rsl"
)

var upgradeRequiredErrors = []error{
	rsl.ErrUnknownRSLEntryType,
	tuf.ErrUnknownRootMetadataVersion,
	tuf.ErrUnknownTargetsMetadataVersion,
}

func IsUpgradeRequiredError(err error) bool {
	for _, target := range upgradeRequiredErrors {
		if errors.Is(err, target) {
			return true
		}
	}

	return false
}
