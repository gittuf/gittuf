// SPDX-License-Identifier: Apache-2.0

package trust

import (
	"github.com/gittuf/gittuf/internal/cmd/trust/addpolicykey"
	"github.com/gittuf/gittuf/internal/cmd/trust/addrootkey"
	i "github.com/gittuf/gittuf/internal/cmd/trust/init"
	"github.com/gittuf/gittuf/internal/cmd/trust/persistent"
	"github.com/gittuf/gittuf/internal/cmd/trust/removepolicykey"
	"github.com/gittuf/gittuf/internal/cmd/trust/removerootkey"
	"github.com/gittuf/gittuf/internal/cmd/trust/sign"
	"github.com/gittuf/gittuf/internal/cmd/trust/updatepolicythreshold"
	"github.com/gittuf/gittuf/internal/cmd/trust/updaterootthreshold"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/apply"
	"github.com/gittuf/gittuf/internal/cmd/trustpolicy/remote"
	"github.com/spf13/cobra"
)

func New() *cobra.Command {
	o := &persistent.Options{}
	cmd := &cobra.Command{
		Use:               "trust",
		Short:             "Tools for gittuf's root of trust",
		DisableAutoGenTag: true,
	}
	o.AddPersistentFlags(cmd)

	cmd.AddCommand(i.New(o))
	cmd.AddCommand(addpolicykey.New(o))
	cmd.AddCommand(addrootkey.New(o))
	cmd.AddCommand(apply.New())
	cmd.AddCommand(remote.New())
	cmd.AddCommand(removepolicykey.New(o))
	cmd.AddCommand(removerootkey.New(o))
	cmd.AddCommand(sign.New(o))
	cmd.AddCommand(updatepolicythreshold.New(o))
	cmd.AddCommand(updaterootthreshold.New(o))

	return cmd
}
