// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package version //nolint:revive

import "runtime/debug"

// gitVersion records the basic version information from Git. It is typically
// overwritten during a go build.
var gitVersion = "devel"

func GetVersion() string {
	buildInfo, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown"
	}

	if buildInfo.Main.Version == "(devel)" || buildInfo.Main.Version == "" {
		return gitVersion
	}

	return buildInfo.Main.Version
}
