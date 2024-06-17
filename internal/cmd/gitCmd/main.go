package main

import (
	"os"
)

func main(){
	Cmd := New()
	if err := Cmd.Execute(); err != nil {
		// We can ignore the linter here (deferred functions are not executed
		// when os.Exit is invoked) because if we do have an error, we don't
		// have a panic, which is what the deferred function is looking for.
		os.Exit(1) //nolint:gocritic
	}
}