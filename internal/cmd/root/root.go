// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"log/slog"
	"os"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/addhooks"
	"github.com/gittuf/gittuf/internal/cmd/attest"
	"github.com/gittuf/gittuf/internal/cmd/clone"
	"github.com/gittuf/gittuf/internal/cmd/dev"
	"github.com/gittuf/gittuf/internal/cmd/policy"
	"github.com/gittuf/gittuf/internal/cmd/profile"
	"github.com/gittuf/gittuf/internal/cmd/rsl"
	"github.com/gittuf/gittuf/internal/cmd/sync"
	"github.com/gittuf/gittuf/internal/cmd/trust"
	"github.com/gittuf/gittuf/internal/cmd/verifymergeable"
	"github.com/gittuf/gittuf/internal/cmd/verifyref"
	"github.com/gittuf/gittuf/internal/cmd/version"
	"github.com/gittuf/gittuf/internal/display"
	"github.com/mattn/go-isatty"
	"github.com/spf13/cobra"
)

type options struct {
	noColor           bool
	verbose           bool
	profile           bool
	cpuProfileFile    string
	memoryProfileFile string
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().BoolVar(
		&o.noColor,
		"no-color",
		false,
		"turn off colored output",
	)

	cmd.PersistentFlags().BoolVar(
		&o.verbose,
		"verbose",
		false,
		"enable verbose logging",
	)

	cmd.PersistentFlags().BoolVar(
		&o.profile,
		"profile",
		false,
		"enable CPU and memory profiling",
	)

	cmd.PersistentFlags().StringVar(
		&o.cpuProfileFile,
		"profile-CPU-file",
		"cpu.prof",
		"file to store CPU profile",
	)

	cmd.PersistentFlags().StringVar(
		&o.memoryProfileFile,
		"profile-memory-file",
		"memory.prof",
		"file to store memory profile",
	)
}

func (o *options) PreRunE(_ *cobra.Command, _ []string) error {
	// Check if colored output must be disabled
	output := os.Stdout
	isTerminal := isatty.IsTerminal(output.Fd()) || isatty.IsCygwinTerminal(output.Fd())
	if o.noColor || !isTerminal {
		display.DisableColor()
	}

	// Setup logging
	level := slog.LevelInfo
	if o.verbose || gittuf.InDebugMode() {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: level,
	})))

	// Start profiling if flag is set
	if o.profile {
		return profile.StartProfiling(o.cpuProfileFile, o.memoryProfileFile)
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "gittuf",
		Short: "A security layer for Git repositories, powered by TUF",
		Long: `gittuf is a command-line interface providing a security layer for Git repositories based on The Update Framework (TUF).

It offers various commands to manage trusted policies, attestations, repository cloning, verification, synchronization, and more.

The CLI supports enhanced security workflows around Git to ensure integrity and authenticity of repository content.

Use the subcommands to perform specific actions such as managing trust policies, adding Git hooks, verifying repository references, and profiling performance.`,

		SilenceUsage:      true,
		DisableAutoGenTag: true,
		PersistentPreRunE: o.PreRunE,
	}

	o.AddFlags(cmd)

	cmd.AddCommand(addhooks.New())
	cmd.AddCommand(attest.New())
	cmd.AddCommand(clone.New())
	cmd.AddCommand(dev.New())
	cmd.AddCommand(trust.New())
	cmd.AddCommand(policy.New())
	cmd.AddCommand(rsl.New())
	cmd.AddCommand(sync.New())
	cmd.AddCommand(verifymergeable.New())
	cmd.AddCommand(verifyref.New())
	cmd.AddCommand(version.New())

	return cmd
}
