// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"time"

	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	tufv03 "github.com/gittuf/gittuf/internal/tuf/v03"
)

// InitializeTargetsMetadata creates a new instance of TargetsMetadata.
func InitializeTargetsMetadata() tuf.TargetsMetadata {
	var targetsMetadata tuf.TargetsMetadata
	if tufv03.AllowV03Metadata() {
		targetsMetadata = tufv03.NewTargetsMetadata()
	} else {
		targetsMetadata = tufv02.NewTargetsMetadata()
	}

	targetsMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))
	return targetsMetadata
}
