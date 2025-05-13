// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/gittuf/gittuf/internal/cmd/profile"
	"github.com/gittuf/gittuf/internal/cmd/root"
)

func main() {
	defer func() {
		if err := profile.StopProfiling(); err != nil {
			fmt.Fprintf(os.Stderr, "unexpected profiling error: %s\n", err.Error())
		}

		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "unexpected error: %s\n\n", fmt.Sprint(r))
			debug.PrintStack()
			fmt.Fprintln(os.Stderr, "\nPlease consider filing a bug on https://github.com/gittuf/gittuf/issues with the stack trace and steps to reproduce this state. Thanks!")

			os.Exit(1) // this is the last possible deferred function to run
		}
	}()

	rootCmd := root.New()
	if err := rootCmd.Execute(); err != nil {
		// We can ignore the linter here (deferred functions are not executed
		// when os.Exit is invoked) because if we do have an error, we don't
		// have a panic, which is what the deferred function is looking for.
		os.Exit(1) //nolint:gocritic
	}
}
