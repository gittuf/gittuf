// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"errors"

	"github.com/gittuf/gittuf/internal/tuf"
	"github.com/gittuf/gittuf/pkg/rsl"
)

// UpgradeGuidance accompanies any error IsUpgradeRequiredError matches.
const UpgradeGuidance = "This repository may use a gittuf feature newer than this client. Upgrade gittuf to the latest release and retry. If this client is already the latest release, the data may be corrupt or malicious and should be reported to the repository owners."

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
