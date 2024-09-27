// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package dev

import (
	"fmt"
	"os"
)

const DevModeKey = "GITTUF_DEV"

var ErrNotInDevMode = fmt.Errorf("this feature is only available in developer mode, and can potentially UNDERMINE repository security; override by setting %s=1", DevModeKey)

// InDevMode returns true if gittuf is currently in developer mode.
func InDevMode() bool {
	return os.Getenv(DevModeKey) == "1"
}
