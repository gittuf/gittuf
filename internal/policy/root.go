// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"time"

	"github.com/gittuf/gittuf/internal/tuf"
	tufv02 "github.com/gittuf/gittuf/internal/tuf/v02"
	tufv03 "github.com/gittuf/gittuf/internal/tuf/v03"
)

// InitializeRootMetadata initializes a new instance of tuf.RootMetadata with
// default values and a given key. The default values are version set to 1,
// expiry date set to one year from now, and the provided key is added.
func InitializeRootMetadata(key tuf.Principal) (tuf.RootMetadata, error) {
	var rootMetadata tuf.RootMetadata
	if tufv03.AllowV03Metadata() {
		rootMetadata = tufv03.NewRootMetadata()
	} else {
		rootMetadata = tufv02.NewRootMetadata()
	}

	rootMetadata.SetExpires(time.Now().AddDate(1, 0, 0).Format(time.RFC3339))

	if err := rootMetadata.AddRootPrincipal(key); err != nil {
		return nil, err
	}

	return rootMetadata, nil
}
