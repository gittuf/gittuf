// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package addhooks

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/gittuf/gittuf/experimental/gittuf"
	"github.com/spf13/cobra"
)

type options struct {
	force     bool
	hookTypes []string
	listHooks bool
}

func (o *options) AddFlags(cmd *cobra.Command) {
	cmd.Flags().BoolVarP(
		&o.force,
		"force",
		"f",
		false,
		"overwrite hooks, if they already exist",
	)

	cmd.Flags().StringSliceVar(
		&o.hookTypes,
		"hooks",
		[]string{"pre-push"},
		"comma-separated list of hook types to install (pre-push)",
	)

	cmd.Flags().BoolVar(
		&o.listHooks,
		"list",
		false,
		"list installed gittuf hooks",
	)
}

func (o *options) Run(cmd *cobra.Command, _ []string) error {
	repo, err := gittuf.LoadRepository(".")
	if err != nil {
		return err
	}

	if o.listHooks {
		return listInstalledHooks(cmd, repo)
	}

	return installHooks(cmd, repo, o.hookTypes, o.force)
}

func installHooks(cmd *cobra.Command, repo *gittuf.Repository, hookTypes []string, force bool) error {
	for _, hookType := range hookTypes {
		hookType = strings.TrimSpace(hookType)

		if hookType != "pre-push" {
			return fmt.Errorf("unsupported hook type: %s", hookType)
		}

		script := generateCrossPlatformScript(hookType, string(prePushScript))

		err := repo.UpdateGitHook(gittuf.HookPrePush, script, force)
		if err != nil {
			var hookErr *gittuf.ErrHookExists
			if !force && errors.As(err, &hookErr) {
				fmt.Fprintf(
					cmd.ErrOrStderr(),
					"'%s' already exists. Use --force flag or merge existing hook and the following script manually:\n\n%s\n",
					string(hookErr.HookType),
					string(script),
				)
				return fmt.Errorf("hook '%s' already exists", hookType)
			} else {
				return fmt.Errorf("failed to install %s hook: %v", hookType, err)
			}
		} else {
			fmt.Fprintf(cmd.OutOrStdout(), "Successfully installed %s hook\n", hookType)
		}
	}

	return nil
}

func listInstalledHooks(cmd *cobra.Command, repo *gittuf.Repository) error {
	fmt.Fprintf(cmd.OutOrStdout(), "Installed gittuf hooks:\n")

	gitRepo := repo.GetGitRepository()
	gitDir := gitRepo.GetGitDir()

	hookPath := filepath.Join(gitDir, "hooks", "pre-push")
	if _, err := os.Stat(hookPath); err == nil {
		fmt.Fprintf(cmd.OutOrStdout(), "  ✓ pre-push\n")
	} else {
		fmt.Fprintf(cmd.OutOrStdout(), "  ✗ pre-push\n")
	}

	return nil
}

func New() *cobra.Command {
	o := &options{}
	cmd := &cobra.Command{
		Use:   "add-hooks",
		Short: "Add git hooks that automatically create and sync RSL",
		Long: `The 'add-hooks' command installs Git hooks that automatically create and sync the RSL when certain Git actions occur, such as a push. By default, it prevents overwriting existing hooks unless the '--force' flag is specified.

Supported hook types:
  - pre-push: Automatically creates RSL entries and syncs with remote before pushing

Examples:
  gittuf add-hooks                    # Install default pre-push hook
  gittuf add-hooks --list             # List installed hooks
  gittuf add-hooks --force            # Force overwrite existing hooks`,
		RunE:              o.Run,
		DisableAutoGenTag: true,
	}
	o.AddFlags(cmd)

	return cmd
}
