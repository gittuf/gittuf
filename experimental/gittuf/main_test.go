// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package gittuf

import (
	"os"
	"testing"
)

// TestMain isolates every test in the package from the developer's global and
// system git config. Hooks, signing, and other settings there otherwise leak
// into the git invocations the tests make. t.Setenv would rule out t.Parallel,
// so the environment is set once for the whole package instead.
func TestMain(m *testing.M) {
	for _, key := range []string{"GIT_CONFIG_GLOBAL", "GIT_CONFIG_SYSTEM"} {
		if err := os.Setenv(key, os.DevNull); err != nil {
			panic(err)
		}
	}

	os.Exit(m.Run())
}
