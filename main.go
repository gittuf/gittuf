package main

import (
	"os"

	"github.com/adityasaky/gittuf/internal/cmd/root"
)

func main() {
	rootCmd := root.New()
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
