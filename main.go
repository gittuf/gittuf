// SPDX-License-Identifier: Apache-2.0

package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/gittuf/gittuf/internal/cmd/root"
)

func main() {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintf(os.Stderr, "unexpected error: %s\n\n", fmt.Sprint(r))
			debug.PrintStack()
			fmt.Fprintln(os.Stderr, "\nPlease consider filing a bug on https:/github.com/gittuf/gittuf/issues with the stack trace and steps to reproduce this state. Thanks!")

			os.Exit(1) // this is the last possible deferred function to run
		}
	}()

	rootCmd := root.New()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
