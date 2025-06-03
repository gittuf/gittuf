// Copyright The gittuf Authors
// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"github.com/gittuf/gittuf/internal/cmd/policy/addkey"
	"github.com/gittuf/gittuf/internal/cmd/policy/addperson"
	"github.com/gittuf/gittuf/internal/cmd/policy/addrule"
	i "github.com/gittuf/gittuf/internal/cmd/policy/init"
	"github.com/gittuf/gittuf/internal/cmd/policy/listprincipals"
	"github.com/gittuf/gittuf/internal/cmd/policy/listrules"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/cmd/policy/removekey"
	"github.com/gittuf/gittuf/internal/cmd/policy/removeperson"
	"github.com/gittuf/gittuf/internal/cmd/policy/removerule"
	"github.com/gittuf/gittuf/internal/cmd/policy/reorderrules"
	"github.com/gittuf/gittuf/internal/cmd/policy/sign"
	"github.com/gittuf/gittuf/internal/cmd/policy/tui"
	"github.com/gittuf/gittuf/internal/cmd/policy/updaterule"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/apply"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/discard"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/remote"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/stage"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	o := &persistent.Options{}
	cmd := &cobra.Command{
		Use:               "policy",
		Short:             "Tools to manage gittuf policies",
		Long:              `The 'policy' command provides a suite of tools for managing gittuf policy configurations. This command serves as a parent for several subcommands that allow users to initialize policy, add or remove principals, view or reorder existing rules and principals, apply, stage, or discard trust policy changes, or interact with policies through a terminal UI.`,
		DisableAutoGenTag: true,
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(addkey.New(o))
	cmd.AddCommand(addperson.New(o))
	cmd.AddCommand(addrule.New(o))
	cmd.AddCommand(apply.New())
	cmd.AddCommand(discard.New())
	cmd.AddCommand(i.New(o))
	cmd.AddCommand(listprincipals.New())
	cmd.AddCommand(listrules.New())
	cmd.AddCommand(remote.New())
	cmd.AddCommand(removekey.New(o))
	cmd.AddCommand(removeperson.New(o))
	cmd.AddCommand(removerule.New(o))
	cmd.AddCommand(reorderrules.New(o))
	cmd.AddCommand(sign.New(o))
	cmd.AddCommand(stage.New())
	cmd.AddCommand(tui.New(o))
	cmd.AddCommand(updaterule.New(o))

	return cmd
}
