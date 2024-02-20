// SPDX-License-Identifier: Apache-2.0

package policy

import (
	"github.com/gittuf/gittuf/internal/cmd/policy/addkey"
	"github.com/gittuf/gittuf/internal/cmd/policy/addrule"
	i "github.com/gittuf/gittuf/internal/cmd/policy/init"
	"github.com/gittuf/gittuf/internal/cmd/policy/listrules"
	"github.com/gittuf/gittuf/internal/cmd/policy/persistent"
	"github.com/gittuf/gittuf/internal/cmd/policy/removerule"
	"github.com/gittuf/gittuf/internal/cmd/policy/sign"
	"github.com/gittuf/gittuf/internal/cmd/policy/updaterule"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/remote"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	o := &persistent.Options{}
	cmd := &cobra.Command{
		Use:               "policy",
		Short:             "Tools to manage gittuf policies",
		DisableAutoGenTag: true,
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(i.New(o))
	cmd.AddCommand(addkey.New(o))
	cmd.AddCommand(addrule.New(o))
	cmd.AddCommand(listrules.New())
	cmd.AddCommand(removerule.New(o))
	cmd.AddCommand(sign.New(o))
	cmd.AddCommand(updaterule.New(o))

	remoteCmd := remote.New()
	cmd.AddCommand(remoteCmd)
	// set signing-key as not required for remote command
	remoteCmd.InheritedFlags().SetAnnotation("signing-key", cobra.BashCompOneRequiredFlag, []string{"false"}) // nolint:errcheck

	return cmd
}
