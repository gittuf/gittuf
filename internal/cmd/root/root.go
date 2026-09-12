// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package root

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/gittuf/gittuf/internal/cmd/addhooks"
	"github.com/gittuf/gittuf/internal/cmd/attest"
	"github.com/gittuf/gittuf/internal/cmd/cache"
	"github.com/gittuf/gittuf/internal/cmd/clone"
	"github.com/gittuf/gittuf/internal/cmd/policy"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/cmd/profile"
	"github.com/gittuf/gittuf/internal/cmd/rsl"
	"github.com/gittuf/gittuf/internal/cmd/sync"
	"github.com/gittuf/gittuf/internal/cmd/trust"
	"github.com/gittuf/gittuf/internal/cmd/tui"
	"github.com/gittuf/gittuf/internal/cmd/verifymergeable"
	"github.com/gittuf/gittuf/internal/cmd/verifynetwork"
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
	storer            string
	storerTrace       bool
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

	cmd.PersistentFlags().StringVar(
		&o.storer,
		"storer",
		string(gittuf.StorerBackendBinary),
		fmt.Sprintf("Git storage backend to use, one of %s or %s (experimental, overrides %s)", gittuf.StorerBackendBinary, gittuf.StorerBackendGoGit, gittuf.StorerBackendEnvKey),
	)

	cmd.PersistentFlags().BoolVar(
		&o.storerTrace,
		"storer-trace",
		false,
		"report Git storage backend call counts, timings and git fork counts to stderr on exit",
	)
}

func (o *options) PreRunE(cmd *cobra.Command, _ []string) error {
	if err := o.selectStorerBackend(cmd); err != nil {
		return err
	}
	gittuf.SetStorerTrace(o.storerTrace)

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
		Use:               "gittuf",
		Short:             "A security layer for Git repositories, powered by TUF",
		Long:              `gittuf is a security layer for Git repositories, powered by TUF. The CLI provides commands to manage gittuf on the repository, including trust management, policy enforcement, signing, verification, and synchronization.`,
		SilenceUsage:      true,
		DisableAutoGenTag: true,
		PersistentPreRunE: o.PreRunE,
	}

	o.AddFlags(cmd)

	cmd.AddCommand(addhooks.New())
	cmd.AddCommand(attest.New())
	cmd.AddCommand(cache.New())
	cmd.AddCommand(clone.New())
	cmd.AddCommand(trust.New())
	cmd.AddCommand(policy.New())
	cmd.AddCommand(rsl.New())
	cmd.AddCommand(sync.New())
	cmd.AddCommand(verifymergeable.New())
	cmd.AddCommand(verifynetwork.New())
	cmd.AddCommand(verifyref.New())
	cmd.AddCommand(version.New())
	cmd.AddCommand(tui.New(&persistent.Options{}))

	return cmd
}

// selectStorerBackend gives an explicit flag precedence over the environment.
func (o *options) selectStorerBackend(cmd *cobra.Command) error {
	backend, err := gittuf.ParseStorerBackend(o.storer)
	if err != nil {
		return err
	}

	// Include flags inherited from the root command.
	var passed bool
	if cmd != nil {
		if flag := cmd.Flag("storer"); flag != nil {
			passed = flag.Changed
		}
	}

	if !passed {
		if fromEnv, err := gittuf.ParseStorerBackend(os.Getenv(gittuf.StorerBackendEnvKey)); err == nil {
			backend = fromEnv
		}
	}

	gittuf.SetStorerBackend(backend)

	return nil
}

// storerTraceReported keeps the trace to one emission. It is reported from
// main's deferred cleanup and again before a non-zero exit, both on the main
// goroutine.
var storerTraceReported bool

// ReportStorerTrace writes the storer trace if one was requested. Only the
// first call writes.
func ReportStorerTrace(w io.Writer) {
	if storerTraceReported {
		return
	}
	storerTraceReported = true

	report, has := gittuf.StorerTraceReport()
	if !has {
		return
	}
	fmt.Fprint(w, report)
}
