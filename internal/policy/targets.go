// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"time"

	"github.com/gittuf/gittuf/internal/tuf"
	tufv01 "github.com/gittuf/gittuf/internal/tuf/v01"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
)

// InitializeTargetsMetadata creates a new instance of TargetsMetadata.
func InitializeTargetsMetadata() tuf.TargetsMetadata {
	var targetsMetadata tuf.TargetsMetadata
	if tufv02.AllowV02Metadata() {
		targetsMetadata = tufv02.NewTargetsMetadata()
	} else {
		targetsMetadata = tufv01.NewTargetsMetadata()
	}

	targetsMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	return targetsMetadata
}
